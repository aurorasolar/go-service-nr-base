package visibility

import (
	"context"
	newrelic "github.com/newrelic/go-agent"
	"go.uber.org/zap"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ProcessRegistry struct {
	xraySuffix string
	app        newrelic.Application
	metrics    MetricsSink

	logger     *zap.Logger
	mtx        sync.Mutex
	numRunning uint64

	rootCtx context.Context
	cancel  context.CancelFunc

	processes     map[string]*ProcessContext
	runningGroups sync.WaitGroup
}

type ProcessContext struct {
	Parent *ProcessRegistry
	Name   string
	Done   chan struct{}
}

func NewProcessRegistry(xraySuffix string, logger *zap.Logger, app newrelic.Application,
	metrics MetricsSink) *ProcessRegistry {

	ctx, cancel := context.WithCancel(context.Background())
	return &ProcessRegistry{
		xraySuffix: xraySuffix,
		app:        app,
		metrics:    metrics,
		logger:     logger,
		rootCtx:    ctx,
		cancel:     cancel,
		processes:  make(map[string]*ProcessContext),
	}
}

func (p *ProcessRegistry) Close() {
	p.logger.Sugar().Infof(
		"Closing the process registry with %d processes running: %s",
		atomic.LoadUint64(&p.numRunning), p.LogRunning())
	p.cancel()
	p.runningGroups.Wait()
	p.logger.Info("Finished waiting for processes to finish")
}

func (p *ProcessRegistry) LogRunning() string {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	var elems []string
	for k := range p.processes {
		elems = append(elems, k)
	}
	sort.Strings(elems)

	return strings.Join(elems, ", ")
}

func (p *ProcessRegistry) HasProcess(name string) bool {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	_, has := p.processes[name]
	return has
}

func (p *ProcessRegistry) CreateProcessContext(name string) ProcessContext {
	return ProcessContext{
		Parent: p,
		Name:   name,
		Done:   make(chan struct{}),
	}
}

func (pc *ProcessContext) prepareRun() {
	p := pc.Parent
	p.mtx.Lock()
	defer p.mtx.Unlock()

	_, has := p.processes[pc.Name]
	if has {
		panic("There's already a process named: " + pc.Name)
	}

	p.processes[pc.Name] = pc
	atomic.AddUint64(&p.numRunning, 1)
	p.runningGroups.Add(1)
}

func (pc *ProcessContext) Run(proc func(ctx context.Context) error) {
	pc.prepareRun()
	go func() {
		defer close(pc.Done)
		defer pc.Parent.markDone(pc.Name)
		xrayName := pc.Name + pc.Parent.xraySuffix

		// Run the process with XRay instrumentation
		_ = RunInstrumented(pc.Parent.rootCtx, xrayName, pc.Parent.app, pc.Parent.metrics,
			pc.Parent.logger, func(xc context.Context) error {

				err := proc(xc)
				if err != nil {
					CL(xc).Error("Async process returned an error", zap.Error(err))
				}
				return err
			})
	}()
}

func (p *ProcessRegistry) markDone(s string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	delete(p.processes, s)
	atomic.AddUint64(&p.numRunning, ^uint64(0))
	p.runningGroups.Done()
}

func (pc *ProcessContext) RunPeriodicProcess(period time.Duration,
	proc func(ctx context.Context) error) {

	pc.prepareRun()

	go func() {
		defer close(pc.Done)
		defer pc.Parent.markDone(pc.Name)

		ticker := time.NewTicker(period)
		defer ticker.Stop()

	loop:
		for {
			xrayName := pc.Name + pc.Parent.xraySuffix

			// Run the process with XRay instrumentation
			_ = RunInstrumented(pc.Parent.rootCtx, xrayName, pc.Parent.app,
				pc.Parent.metrics, pc.Parent.logger, func(xc context.Context) error {

					err := proc(xc)
					if err != nil {
						CL(xc).Error("Async process returned an error", zap.Error(err))
					}
					return err
				})

			select {
			case <-ticker.C:
			case <-pc.Parent.rootCtx.Done():
				break loop
			}
		}
	}()
}

func (pc *ProcessContext) Wait() {
	<-pc.Done
}

func (p *ProcessRegistry) GetWaitChannel(processName string) <-chan struct{} {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	proc := p.processes[processName]
	if proc == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return proc.Done
}
