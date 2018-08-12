package params_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/go-playground/validator.v9"

	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/t/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNodeConfigWithDefaults(t *testing.T) {
	c, err := params.NewNodeConfigWithDefaults(
		"/some/data/path",
		params.RopstenNetworkID,
		params.WithFleet(params.FleetBeta),
		params.WithLES(),
		params.WithMailserver(),
	)
	require.NoError(t, err)
	assert.Equal(t, "/some/data/path", c.DataDir)
	assert.Equal(t, "/some/data/path/keystore", c.KeyStoreDir)
	// assert Whisper
	assert.Equal(t, true, c.WhisperConfig.Enabled)
	assert.Equal(t, "/some/data/path/wnode", c.WhisperConfig.DataDir)
	assert.Equal(t, true, c.WhisperConfig.EnableNTPSync)
	// assert MailServer
	assert.Equal(t, true, c.WhisperConfig.EnableMailServer)
	assert.NotEmpty(t, c.WhisperConfig.MailServerPassword)
	// assert cluster
	assert.Equal(t, false, c.NoDiscovery)
	assert.Equal(t, params.FleetBeta, c.ClusterConfig.Fleet)
	assert.NotEmpty(t, c.ClusterConfig.BootNodes)
	assert.NotEmpty(t, c.ClusterConfig.StaticNodes)
	assert.NotEmpty(t, c.ClusterConfig.RendezvousNodes)
	// assert LES
	assert.Equal(t, true, c.LightEthConfig.Enabled)
	// assert peers limits
	assert.Contains(t, c.RequireTopics, params.WhisperDiscv5Topic)
	assert.Contains(t, c.RequireTopics, discv5.Topic(params.LesTopic(int(c.NetworkID))))
	// assert other
	assert.Equal(t, false, c.HTTPEnabled)
	assert.Equal(t, false, c.IPCEnabled)

}

func TestNewConfigFromJSON(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "geth-config-tests")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) // nolint: errcheck

	c, err := params.NewConfigFromJSON(`{
		"NetworkId": 3,
		"DataDir": "` + tmpDir + `",
		"NoBackupDataDir": "` + tmpDir + `",
		"KeyStoreDir": "` + tmpDir + `",
		"NoDiscovery": true
	}`)
	require.NoError(t, err)
	require.Equal(t, uint64(3), c.NetworkID)
	require.Equal(t, tmpDir, c.DataDir)
	require.Equal(t, tmpDir, c.KeyStoreDir)
}

func TestConfigWriteRead(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "geth-config-tests")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir) // nolint: errcheck

	nodeConfig, err := utils.MakeTestNodeConfigWithDataDir("", tmpDir, params.RopstenNetworkID)
	require.Nil(t, err, "cannot create new config object")

	err = nodeConfig.Save()
	require.Nil(t, err, "cannot persist configuration")

	loadedConfigData, err := ioutil.ReadFile(filepath.Join(nodeConfig.DataDir, "config.json"))
	require.Nil(t, err, "cannot read configuration from disk")
	loadedConfig := string(loadedConfigData)
	require.Contains(t, loadedConfig, fmt.Sprintf(`"NetworkId": %d`, params.RopstenNetworkID))
	require.Contains(t, loadedConfig, fmt.Sprintf(`"DataDir": "%s"`, tmpDir))
	require.Contains(t, loadedConfig, fmt.Sprintf(`"NoBackupDataDir": "%s"`, tmpDir))
}

// TestNodeConfigValidate checks validation of individual fields.
func TestNodeConfigValidate(t *testing.T) {
	testCases := []struct {
		Name        string
		Config      string
		Error       string
		FieldErrors map[string]string // map[Field]Tag
		CheckFunc   func(*testing.T, *params.NodeConfig)
	}{
		{
			Name: "Valid JSON config",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/tmp/data",
				"NoBackupDataDir": "/tmp/data",
				"KeyStoreDir": "/tmp/data",
				"NoDiscovery": true
			}`,
		},
		{
			Name:   "Invalid JSON config",
			Config: `{"NetworkId": }`,
			Error:  "invalid character '}'",
		},
		{
			Name:   "Invalid field type",
			Config: `{"NetworkId": "abc"}`,
			Error:  "json: cannot unmarshal string into Go struct field",
		},
		{
			Name:   "Validate all required fields",
			Config: `{}`,
			FieldErrors: map[string]string{
				"NetworkID":       "required",
				"DataDir":         "required",
				"NoBackupDataDir": "required",
				"KeyStoreDir":     "required",
			},
		},
		{
			Name: "Validate that Name does not contain slash",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"Name": "invalid/name"
			}`,
			FieldErrors: map[string]string{
				"Name": "excludes",
			},
		},
		{
			Name: "Validate that NodeKey is checked for validity",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"NodeKey": "foo"
			}`,
			Error: "NodeKey is invalid",
		},
		{
			Name: "Validate that UpstreamConfig.URL is validated if UpstreamConfig is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"UpstreamConfig": {
					"Enabled": true,
					"URL": "[bad.url]"
				}
			}`,
			Error: "'[bad.url]' is invalid",
		},
		{
			Name: "Validate that UpstreamConfig.URL is not validated if UpstreamConfig is disabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"UpstreamConfig": {
					"Enabled": false,
					"URL": "[bad.url]"
				}
			}`,
		},
		{
			Name: "Validate that UpstreamConfig.URL validation passes if UpstreamConfig.URL is valid",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"UpstreamConfig": {
					"Enabled": true,
					"URL": "` + params.MainnetEthereumNetworkURL + `"
				}
			}`,
		},
		{
			Name: "Validate that ClusterConfig.Fleet is verified to not be empty if ClusterConfig is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"ClusterConfig": {
					"Enabled": true
				}
			}`,
			Error: "ClusterConfig.Fleet is empty",
		},
		{
			Name: "Validate that ClusterConfig.BootNodes is verified to not be empty if discovery is disabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": false
			}`,
			Error: "NoDiscovery is false, but ClusterConfig.BootNodes is empty",
		},
		{
			Name: "Validate that ClusterConfig.RendezvousNodes is verified to be empty if Rendezvous is disabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"Rendezvous": true
			}`,
			Error: "Rendezvous is enabled, but ClusterConfig.RendezvousNodes is empty",
		},
		{
			Name: "Validate that ClusterConfig.RendezvousNodes is verified to contain nodes if Rendezvous is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"Rendezvous": false,
				"ClusterConfig": {
					"RendezvousNodes": ["a"]
				}
			}`,
			Error: "Rendezvous is disabled, but ClusterConfig.RendezvousNodes is not empty",
		},
		{
			Name: "Validate that WhisperConfig.DataDir is checked to not be empty if mailserver is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"WhisperConfig": {
					"Enabled": true,
					"EnableMailServer": true,
					"MailserverPassword": "foo"
				}
			}`,
			Error: "WhisperConfig.DataDir must be specified when WhisperConfig.EnableMailServer is true",
		},
		{
			Name: "Validate that check for WhisperConfig.DataDir passes if it is not empty and mailserver is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"WhisperConfig": {
					"Enabled": true,
					"EnableMailServer": true,
					"DataDir": "/foo",
					"MailserverPassword": "foo"
				}
			}`,
			CheckFunc: func(t *testing.T, config *params.NodeConfig) {
				require.Equal(t, "foo", config.WhisperConfig.MailServerPassword)
			},
		},
		{
			Name: "Validate that WhisperConfig.DataDir is checked to not be empty if mailserver is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"WhisperConfig": {
					"Enabled": true,
					"EnableMailServer": true,
					"MailserverPassword": "foo"
				}
			}`,
			Error: "WhisperConfig.DataDir must be specified when WhisperConfig.EnableMailServer is true",
		},
		{
			Name: "Validate that WhisperConfig.MailserverPassword and WhisperConfig.MailServerAsymKey are checked to not be empty if mailserver is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"WhisperConfig": {
					"Enabled": true,
					"EnableMailServer": true,
					"DataDir": "/foo"
				}
			}`,
			Error: "WhisperConfig.MailServerPassword or WhisperConfig.MailServerAsymKey must be specified when WhisperConfig.EnableMailServer is true",
		},
		{
			Name: "Validate that WhisperConfig.MailServerAsymKey is checked to not be empty if mailserver is enabled",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"WhisperConfig": {
					"Enabled": true,
					"EnableMailServer": true,
					"DataDir": "/foo",
					"MailServerAsymKey": "06c365919f1fc8e13ff79a84f1dd14b7e45b869aa5fc0e34940481ee20d32f90"
				}
			}`,
			CheckFunc: func(t *testing.T, config *params.NodeConfig) {
				require.Equal(t, "06c365919f1fc8e13ff79a84f1dd14b7e45b869aa5fc0e34940481ee20d32f90", config.WhisperConfig.MailServerAsymKey)
			},
		},
		{
			Name: "Validate that WhisperConfig.MailServerAsymKey is checked for validity",
			Config: `{
				"NetworkId": 1,
				"DataDir": "/some/dir",
				"NoBackupDataDir": "/some/dir",
				"KeyStoreDir": "/some/dir",
				"NoDiscovery": true,
				"WhisperConfig": {
					"Enabled": true,
					"EnableMailServer": true,
					"DataDir": "/foo",
					"MailServerAsymKey": "bar"
				}
			}`,
			Error: "WhisperConfig.MailServerAsymKey is invalid",
		},
	}

	for _, tc := range testCases {
		t.Logf("Test Case %s", tc.Name)

		config, err := params.NewConfigFromJSON(tc.Config)

		switch err := err.(type) {
		case validator.ValidationErrors:
			for _, ve := range err {
				require.Contains(t, tc.FieldErrors, ve.Field())
				require.Equal(t, tc.FieldErrors[ve.Field()], ve.Tag())
			}
		case error:
			if tc.Error == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, err.Error(), tc.Error)
			}
		case nil:
			if tc.Error != "" {
				require.Error(t, err, "Error should be '%v'", tc.Error)
			}
			require.Nil(t, tc.FieldErrors)
			if tc.CheckFunc != nil {
				tc.CheckFunc(t, config)
			}
		}
	}
}
