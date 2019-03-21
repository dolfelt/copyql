package data

import (
	"strings"

	"github.com/spf13/viper"
)

// SQLConnection handles SQL configuration
type SQLConnection struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// Configuration holds all the configuration
type Configuration struct {
	Source      SQLConnection
	Destination SQLConnection
	Relations   map[string]string
	SkipTables  []string `mapstructure:"skip"`
	FileIn      string
	FileOut     string
	Verbose     bool
}

// LoadConfig loads the configuration for copying data
func LoadConfig(configFile string) (*Configuration, error) {
	setConfigDefaults()

	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("copyql")
		viper.AddConfigPath(".")
	}

	viper.SetEnvPrefix("copyql")
	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.ReadInConfig()

	var config Configuration
	err := viper.Unmarshal(&config)

	if err != nil {
		return nil, err
	}

	return &config, nil
}

func setConfigDefaults() {
	for _, db := range []string{"Source", "Destination"} {
		viper.SetDefault(db+".Port", 3306)
		viper.SetDefault(db+".Host", "localhost")
		viper.SetDefault(db+".User", "root")
	}
	viper.SetDefault("SkipTables", []string{})
}
