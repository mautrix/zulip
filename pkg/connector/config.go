package connector

import (
	_ "embed"

	up "go.mau.fi/util/configupgrade"
	"gopkg.in/yaml.v3"
)

//go:embed example-config.yaml
var ExampleConfig string

type Config struct {
}

func (zc *ZulipConnector) GetConfig() (example string, data any, upgrader up.Upgrader) {
	return ExampleConfig, &zc.Config, &up.StructUpgrader{
		SimpleUpgrader: up.SimpleUpgrader(upgradeConfig),
		Blocks:         [][]string{},
		Base:           ExampleConfig,
	}
}

type umConfig Config

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	err := node.Decode((*umConfig)(c))
	if err != nil {
		return err
	}
	return c.PostProcess()
}

func (c *Config) PostProcess() (err error) {
	return
}

func upgradeConfig(helper up.Helper) {

}
