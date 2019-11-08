package xraysql

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/rdsutils"
	"github.com/jackc/pgx/v4/stdlib"
	"github.com/aurorasolar/go-service-base/utils"
	"reflect"
	"strings"
)

type PgConnectorWithRds struct {
	config aws.Config

	rdsDbInstance, sslCaPath string
	rdsDb, rdsUser           string

	rdsEndpointAddr string
	rdsEndpointPort int64

	isRds      bool
	connString string
}

// Create a Postgres connector to use with XrayConnector. The PgConnector supports
// resolving RDS endpoints and IAM authentication.
func MakePgConnector(ctx context.Context, connStr string, sslCaPath string,
	config aws.Config) (*PgConnectorWithRds, error) {

	// Not an RDS-format connection string
	if !strings.HasPrefix(connStr, "rds:") {
		res := &PgConnectorWithRds{
			isRds:      false,
			connString: connStr,
		}
		return res, nil
	}

	// Split the connection string
	splits := strings.Split(connStr, ":")
	if len(splits) != 4 {
		return nil, fmt.Errorf("bad RDS connection string %s", connStr)
	}

	res := &PgConnectorWithRds{
		isRds:         true,
		config:        config,
		connString:    connStr,
		rdsDbInstance: splits[1],
		rdsDb:         splits[2],
		rdsUser:       splits[3],
		sslCaPath:     sslCaPath,
	}

	rdsEndpointAddr, rdsEndpointPort, err := resolveRdsEndpoint(ctx,
		config, res.rdsDbInstance)
	if err != nil {
		return nil, err
	}
	res.rdsEndpointAddr = rdsEndpointAddr
	res.rdsEndpointPort = rdsEndpointPort

	return res, nil
}

// Get the connection endpoint from the database name
func resolveRdsEndpoint(ctx context.Context, config aws.Config,
	dbInstanceName string) (string, int64, error) {

	rdsCli := rds.New(config)
	dbis, err := rdsCli.DescribeDBInstancesRequest(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(dbInstanceName),
	}).Send(ctx)
	if err != nil {
		return "", 0, err
	}

	addr := *dbis.DBInstances[0].Endpoint.Address
	port := *dbis.DBInstances[0].Endpoint.Port
	return addr, port, nil
}

func (pc *PgConnectorWithRds) getConnString(ctx context.Context) (string, error) {
	if !pc.isRds {
		return pc.connString, nil
	}

	token, err := rdsutils.BuildAuthToken(
		fmt.Sprintf("%s:%d", pc.rdsEndpointAddr, pc.rdsEndpointPort),
		pc.config.Region, pc.rdsUser, pc.config.Credentials)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("host=%s port=%d database=%s "+
		"user=%s sslmode=verify-full sslrootcert=%s password=%s",
		pc.rdsEndpointAddr, pc.rdsEndpointPort, pc.rdsDb, pc.rdsUser, pc.sslCaPath, token), nil
}

func (pc *PgConnectorWithRds) Connect(ctx context.Context) (driver.Conn,
	*ConnInfo, error) {

	connString, err := pc.getConnString(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Open the Postgresql connection!
	drv := stdlib.Driver{}
	conn, err := drv.Open(connString)
	if err != nil {
		return nil, nil, err
	}
	cl := utils.NewCleanupErr(conn.Close)
	defer cl.Cleanup()

	connInfo, err := pc.readDriverData(ctx, connString, conn)
	if err != nil {
		return nil, nil, err
	}

	cl.Disarm()
	return conn, connInfo, nil
}

func (pc *PgConnectorWithRds) readDriverData(ctx context.Context, connString string,
	conn driver.Conn) (*ConnInfo, error) {

	t := reflect.TypeOf(conn)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	res := ConnInfo{}
	res.DriverName = t.PkgPath()

	rows, err := conn.(driver.QueryerContext).QueryContext(ctx,
		"SELECT version(), current_user, current_database()", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	vals := make([]driver.Value, 3)
	err = rows.Next(vals)
	if err != nil {
		return nil, err
	}

	res.DbType = "postgres"
	res.DbVersion = vals[0].(string)
	res.DbUser = vals[1].(string)
	res.DbName = vals[2].(string)

	res.SanitizedConnString = fmt.Sprintf(
		"postgresql://%s:****@%s:%d/%s", res.DbUser,
		pc.rdsEndpointAddr, pc.rdsEndpointPort, res.DbName)

	return &res, nil
}
