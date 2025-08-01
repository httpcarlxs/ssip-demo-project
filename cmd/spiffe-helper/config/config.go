package config

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/token"
	"github.com/sirupsen/logrus"
	"github.com/spiffe/spiffe-helper/pkg/health"
	"github.com/spiffe/spiffe-helper/pkg/sidecar"
)

const (
	defaultAgentAddress      = "/tmp/spire-agent/public/api.sock"
	defaultCertFileMode      = 0644
	defaultKeyFileMode       = 0600
	defaultJWTBundleFileMode = 0600
	defaultJWTSVIDFileMode   = 0600
	defaultBindPort          = 8081
	defaultLivenessPath      = "/live"
	defaultReadinessPath     = "/ready"
)

type Config struct {
	AddIntermediatesToBundle bool          `hcl:"add_intermediates_to_bundle"`
	AgentAddress             string        `hcl:"agent_address"`
	Cmd                      string        `hcl:"cmd"`
	CmdArgs                  string        `hcl:"cmd_args"`
	PIDFilename              string        `hcl:"pid_file_name"`
	CertDir                  string        `hcl:"cert_dir"`
	CertFileMode             int           `hcl:"cert_file_mode"`
	KeyFileMode              int           `hcl:"key_file_mode"`
	JWTBundleFileMode        int           `hcl:"jwt_bundle_file_mode"`
	JWTSVIDFileMode          int           `hcl:"jwt_svid_file_mode"`
	IncludeFederatedDomains  bool          `hcl:"include_federated_domains"`
	RenewSignal              string        `hcl:"renew_signal"`
	DaemonMode               *bool         `hcl:"daemon_mode"`
	HealthCheck              health.Config `hcl:"health_checks"`
	Hint                     string        `hcl:"hint"`
	ParallelRequests         int           `hcl:"parallel_requests"`

	// x509 configuration
	SVIDFilename       string `hcl:"svid_file_name"`
	SVIDKeyFilename    string `hcl:"svid_key_file_name"`
	SVIDBundleFilename string `hcl:"svid_bundle_file_name"`

	// JWT configuration
	JWTSVIDs          []JWTConfig `hcl:"jwt_svids"`
	JWTBundleFilename string      `hcl:"jwt_bundle_file_name"`

	UnusedKeyPositions map[string][]token.Pos `hcl:",unusedKeyPositions"`
}

type JWTConfig struct {
	JWTAudience       string   `hcl:"jwt_audience"`
	JWTExtraAudiences []string `hcl:"jwt_extra_audiences"`
	JWTSVIDFilename   string   `hcl:"jwt_svid_file_name"`

	UnusedKeyPositions map[string][]token.Pos `hcl:",unusedKeyPositions"`
}

// ... (ParseConfigFile, ParseConfigFlagOverrides, ValidateConfig, etc. remain the same)

func NewSidecarConfig(config *Config, log logrus.FieldLogger) *sidecar.Config {
	sidecarConfig := &sidecar.Config{
		AddIntermediatesToBundle: config.AddIntermediatesToBundle,
		AgentAddress:             config.AgentAddress,
		Cmd:                      config.Cmd,
		CmdArgs:                  config.CmdArgs,
		PIDFilename:              config.PIDFilename,
		CertDir:                  config.CertDir,
		CertFileMode:             fs.FileMode(config.CertFileMode),
		KeyFileMode:              fs.FileMode(config.KeyFileMode),
		JWTBundleFileMode:        fs.FileMode(config.JWTBundleFileMode),
		JWTSVIDFileMode:          fs.FileMode(config.JWTSVIDFileMode),
		IncludeFederatedDomains:  config.IncludeFederatedDomains,
		JWTBundleFilename:        config.JWTBundleFilename,
		Log:                      log,
		RenewSignal:              config.RenewSignal,
		SVIDFilename:             config.SVIDFilename,
		SVIDKeyFilename:          config.SVIDKeyFilename,
		SVIDBundleFilename:       config.SVIDBundleFilename,
		ParallelRequests:         config.ParallelRequests,
		Hint:                     config.Hint,
	}

	for _, jwtSVID := range config.JWTSVIDs {
		sidecarConfig.JWTSVIDs = append(sidecarConfig.JWTSVIDs, sidecar.JWTConfig{
			JWTAudience:       jwtSVID.JWTAudience,
			JWTExtraAudiences: jwtSVID.JWTExtraAudiences,
			JWTSVIDFilename:   jwtSVID.JWTSVIDFilename,
		})
	}

	return sidecarConfig
}

func validateX509Config(c *Config) (bool, error) {
	x509EmptyCount := countEmpty(c.SVIDFilename, c.SVIDBundleFilename, c.SVIDKeyFilename)
	if x509EmptyCount != 0 && x509EmptyCount != 3 {
		return false, errors.New("all or none of 'svid_file_name', 'svid_key_file_name', 'svid_bundle_file_name' must be specified")
	}

	return x509EmptyCount == 0, nil
}

func validateJWTConfig(c *Config) (bool, bool) {
	jwtBundleEmptyCount := countEmpty(c.JWTBundleFilename)

	return jwtBundleEmptyCount == 0, len(c.JWTSVIDs) > 0
}

func countEmpty(configs ...string) int {
	cnt := 0
	for _, config := range configs {
		if config == "" {
			cnt++
		}
	}

	return cnt
}

// isFlagPassed tests to see if a command line argument was set at all or left empty
func isFlagPassed(name string) bool {
	var found bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
}

// mapKeysToString returns a comma separated string with all the keys from a map
func mapKeysToString[V any](myMap map[string]V) string {
	keys := make([]string, 0, len(myMap))
	for key := range myMap {
		keys = append(keys, key)
	}

	slices.Sort(keys)
	return strings.Join(keys, ",")
}
