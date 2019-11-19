package nrsql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/lib/pq"
	"github.com/newrelic/go-agent/_integrations/nrpq"
	"strings"
	"sync"
	"time"
)

const MaxRdsRetriesSec = 5

type PgConnectorWithRds struct {
	config aws.Config
	isRds  bool

	rdsDb, postgresDbName string
	sslCaPath             string

	mtx        sync.Mutex
	connString string
	delegate   driver.Connector
}

// Example secret structure:
// {
//   "username": "rds",
//   "engine": "postgres",
//   "host": "xyzzy-db.cbimanaxd4pt.us-east-2.rds.amazonaws.com",
//   "password": "7f*w3-oOZ$17,g&b_uVIH;N^Or]=H7<>",
//   "port": 5432, "dbInstanceIdentifier": "xyzzy-db"
// }
type connInfo struct {
	Username, Password string
	Host               string
	Port               int32
}

// Create a Postgres connector to use with NewRelic. The PgConnector supports
// resolving RDS endpoints and AWS secrets-based authentication.
func MakePgConnector(ctx context.Context, connStr string, sslCaPath string,
	config aws.Config) (*PgConnectorWithRds, error) {

	// Not an RDS-format connection string
	if !strings.HasPrefix(connStr, "rds:") {
		connector, err := pq.NewConnector(connStr)
		if err != nil {
			return nil, err
		}

		res := &PgConnectorWithRds{
			isRds:      false,
			connString: connStr,
			delegate:   connector,
		}
		return res, nil
	}

	// Split the connection string
	splits := strings.Split(connStr, ":")
	if len(splits) != 3 {
		return nil, fmt.Errorf("bad RDS connection string %s", connStr)
	}

	res := &PgConnectorWithRds{
		isRds:          true,
		config:         config,
		connString:     connStr,
		rdsDb:          splits[1],
		postgresDbName: splits[2],
		sslCaPath:      sslCaPath,
	}

	err := res.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (pc *PgConnectorWithRds) getCurrentConnInfo(ctx context.Context) (*connInfo, error) {
	secretName := fmt.Sprintf("db/%s", pc.rdsDb)

	sm := secretsmanager.New(pc.config)

	//Create a Secrets Manager client
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
		// VersionStage defaults to AWSCURRENT if unspecified
		VersionStage: aws.String("AWSCURRENT"),
	}

	result, err := sm.GetSecretValueRequest(input).Send(ctx)
	if err != nil {
		return nil, err
	}

	if result.SecretString == nil || *result.SecretString == "" {
		return nil, fmt.Errorf("no string secret")
	}

	info := connInfo{}
	err = json.Unmarshal([]byte(*result.SecretString), &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (pc *PgConnectorWithRds) getConnString(info *connInfo) string {
	return fmt.Sprintf("host=%s port=%d database=%s "+
		"user=%s sslmode=verify-full sslrootcert=%s password=%s",
		info.Host, info.Port, pc.postgresDbName, info.Username, pc.sslCaPath,
		info.Password)
}

func (pc *PgConnectorWithRds) Driver() driver.Driver {
	return pc.Driver()
}

func (pc *PgConnectorWithRds) tryConnection(ctx context.Context) (driver.Conn, error) {
	pc.mtx.Lock()
	defer pc.mtx.Unlock()

	if pc.delegate != nil {
		conn, err := pc.delegate.Connect(ctx)
		if err == nil {
			return conn, nil
		}
		pc.delegate = nil
	}

	info, err := pc.getCurrentConnInfo(ctx)
	if err != nil {
		return nil, err
	}

	connector, err := nrpq.NewConnector(pc.getConnString(info))
	if err != nil {
		return nil, err
	}
	conn, err := connector.Connect(ctx)
	if err == nil {
		pc.delegate = connector
		return conn, nil
	}

	return nil, err
}

func (pc *PgConnectorWithRds) Connect(ctx context.Context) (driver.Conn, error) {
	if !pc.isRds {
		return pc.delegate.Connect(ctx)
	}

	// A small retry loop to compensate for the possibility of secret rotation
	start := time.Now().Unix()
	for ;; {
		conn, err := pc.tryConnection(ctx)
		if err == nil {
			return conn, err
		}

		if time.Now().Unix()-start > MaxRdsRetriesSec {
			return nil, err
		}

		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		default:
		}
	}
}

func (pc *PgConnectorWithRds) Ping(ctx context.Context) error {
	conn, err := pc.Connect(ctx)
	if err != nil {
		return err
	}
	//noinspection GoUnhandledErrorResult
	defer conn.Close()

	return conn.(driver.Pinger).Ping(ctx)
}
