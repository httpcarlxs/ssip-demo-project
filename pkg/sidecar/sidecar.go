package sidecar

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/spiffe/spiffe-helper/pkg/disk"
	"github.com/spiffe/spiffe-helper/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Event hooks used by unit tests to coordinate goroutines
type hooks struct {
	certReady        func(svids *workloadapi.X509Context)
	cmdExit          func(os.ProcessState)
	pidFileSignalled func(pid int, err error)
}

// Sidecar is the component that consumes the Workload API and renews certs
type Sidecar struct {
	config         *Config
	client         *workloadapi.Client
	jwtSource      *workloadapi.JWTSource
	processRunning bool
	process        *os.Process

	// Mutex to protect processRunning
	mu sync.Mutex

	// Health server
	health Health

	// stdio to connect to the 'cmd' to run. These are used in tests to
	// capture and/or redirect I/O from the guest command. In future they
	// could also be exposed via Config to allow a user of this package to
	// redirect I/O in custom sidecars. These have the same semantics as
	// https://pkg.go.dev/os/exec#Cmd
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	// Used for synchronization in unit tests
	hooks hooks
}

type Health struct {
	FileWriteStatuses FileWriteStatuses `json:"file_write_statuses"`
}

type FileWriteStatuses struct {
	X509WriteStatus *string           `json:"x509_write_status,omitempty"`
	JWTWriteStatus  map[string]string `json:"jwt_write_status"`
}

const (
	writeStatusUnwritten = "unwritten"
	writeStatusFailed    = "failed"
	writeStatusWritten   = "written"
)

// New creates a new SPIFFE sidecar
func New(config *Config) *Sidecar {
	s := &Sidecar{
		config: config,
		health: Health{
			FileWriteStatuses: FileWriteStatuses{
				JWTWriteStatus: make(map[string]string),
			},
		},
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		hooks: hooks{
			certReady:        func(*workloadapi.X509Context) {},
			cmdExit:          func(os.ProcessState) {},
			pidFileSignalled: func(int, error) {},
		},
	}

	s.setupHealth()
	return s
}

func (s *Sidecar) setupHealth() {
	if s.x509Enabled() {
		writeStatus := writeStatusUnwritten
		s.health.FileWriteStatuses.X509WriteStatus = &writeStatus
	}
	if s.jwtBundleEnabled() {
		jwtBundleFilePath := path.Join(s.config.CertDir, s.config.JWTBundleFilename)
		s.health.FileWriteStatuses.JWTWriteStatus[jwtBundleFilePath] = writeStatusUnwritten
	}
	for _, jwtConfig := range s.config.JWTSVIDs {
		jwtSVIDFilename := path.Join(s.config.CertDir, jwtConfig.JWTSVIDFilename)
		s.health.FileWriteStatuses.JWTWriteStatus[jwtSVIDFilename] = writeStatusUnwritten
	}
}

// RunDaemon starts the main loop
func (s *Sidecar) RunDaemon(ctx context.Context) error {
	if err := s.setupClients(ctx); err != nil {
		return err
	}
	if s.client != nil {
		defer s.client.Close()
	}
	if s.jwtSource != nil {
		defer s.jwtSource.Close()
	}

	var tasks []func(context.Context) error

	// If parallel requests are configured, run the parallel daemon loop.
	// Otherwise, run the standard watcher-based daemon.
	if s.config.ParallelRequests > 0 {
		s.config.Log.Info("Starting in continuous parallel request mode")
		tasks = append(tasks, s.runParallelDaemon)
	} else {
		s.config.Log.Info("Starting in standard daemon mode")
		if s.x509Enabled() {
			s.config.Log.Info("Watching for X509 Context")
			tasks = append(tasks, s.watchX509Context)
		}
		if s.jwtBundleEnabled() {
			s.config.Log.Info("Watching for JWT Bundles")
			tasks = append(tasks, s.watchJWTBundles)
		}
		if s.jwtSVIDsEnabled() {
			tasks = append(tasks, s.watchJWTSVIDs)
		}
	}

	err := util.RunTasks(ctx, tasks...)
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}

// runParallelDaemon starts a pool of workers that continuously make fetch requests.
func (s *Sidecar) runParallelDaemon(ctx context.Context) error {
	var wg sync.WaitGroup

	// Start a pool of N workers, where N is ParallelRequests.
	for i := 0; i < s.config.ParallelRequests; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.config.Log.Debugf("Starting parallel worker %d", workerID)

			// Each worker loops indefinitely, making requests.
			for {
				// Perform the fetch operation. Errors are logged within the function.
				_ = s.fetchAllCredentials(ctx)

				// Check if the context has been canceled after the work is done.
				// If so, the worker should exit its loop.
				select {
				case <-ctx.Done():
					s.config.Log.Debugf("Stopping parallel worker %d", workerID)
					return
				default:
					// Continue to the next request immediately.
				}
			}
		}(i + 1)
	}

	// Wait for the context to be canceled (e.g., by SIGINT).
	<-ctx.Done()

	// Wait for all worker goroutines to finish their current request and exit.
	s.config.Log.Info("Shutdown signal received, waiting for parallel workers to stop...")
	wg.Wait()
	s.config.Log.Info("All parallel workers stopped.")

	return nil
}

// Run is for one-shot mode. It makes a single burst of parallel requests if configured.
func (s *Sidecar) Run(ctx context.Context) error {
	if err := s.setupClients(ctx); err != nil {
		return err
	}
	if s.client != nil {
		defer s.client.Close()
	}
	if s.jwtSource != nil {
		defer s.jwtSource.Close()
	}

	if s.config.ParallelRequests > 0 {
		s.config.Log.Infof("Running a burst of %d parallel requests", s.config.ParallelRequests)
		return util.RunTasksInParallel(ctx, s.fetchAllCredentials, s.config.ParallelRequests)
	}

	return s.fetchAllCredentials(ctx)
}

func (s *Sidecar) fetchAllCredentials(ctx context.Context) error {
	if s.x509Enabled() {
		s.config.Log.Debug("Fetching x509 certificates")
		if err := s.fetchAndWriteX509Context(ctx); err != nil {
			s.config.Log.WithError(err).Error("Error fetching x509 certificates")
			return err
		}
		s.config.Log.Info("Successfully fetched x509 certificates")
	}

	if s.jwtBundleEnabled() {
		s.config.Log.Debug("Fetching JWT Bundle")
		if err := s.fetchAndWriteJWTBundle(ctx); err != nil {
			s.config.Log.WithError(err).Error("Error fetching JWT bundle")
			return err
		}
		s.config.Log.Info("Successfully fetched JWT bundle")
	}

	if s.jwtSVIDsEnabled() {
		s.config.Log.Debug("Fetching JWT SVIDs")
		if err := s.fetchAndWriteJWTSVIDs(ctx); err != nil {
			s.config.Log.WithError(err).Error("Error fetching JWT SVIDs")
			return err
		}
		s.config.Log.Info("Successfully fetched JWT SVIDs")
	}

	return nil
}

// ... (the rest of the file remains the same)
