package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/dolfelt/copyql/data"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configFile string

func init() {
	RootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is ./copyql.yaml)")

	RootCmd.PersistentFlags().String("in", "", "json file to read into the destination data store")
	viper.BindPFlag("FileIn", RootCmd.PersistentFlags().Lookup("in"))

	RootCmd.PersistentFlags().String("out", "", "json file to write the data from the source into")
	viper.BindPFlag("FileOut", RootCmd.PersistentFlags().Lookup("out"))

}

// RootCmd controls the entry into the application
var RootCmd = &cobra.Command{
	Use:   "copyql [table.column:value]",
	Short: "Copy data and relationships from one SQL store to another",
	Run:   rootRun,
}

func rootRun(cmd *cobra.Command, args []string) {
	config, err := data.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(config)
	var pkg data.TableData

	if len(config.FileIn) == 0 {
		dbSource := connect(config.Source)
		defer dbSource.Close()

		copy := data.CopyQL{
			DB: dbSource,
		}

		relations, err := copy.BuildRelations()
		if err != nil {
			log.Fatal(err)
		}

		pkg := copy.GetData(data.Pointer{
			Table:  "accounts",
			Column: "id",
		}, 10000, *relations)

		// jsonString, _ := json.MarshalIndent(relations, "", "  ")
		// fmt.Println(string(jsonString))
		tmpFile := config.FileOut
		if len(tmpFile) == 0 {
			tmpFile = "output.json"
		}
		jsonString, _ := json.MarshalIndent(pkg, "", "  ")
		err = ioutil.WriteFile(tmpFile, jsonString, 0644)
		if err != nil {
			log.Fatal(err)
		}

		if len(config.FileOut) > 0 {
			os.Exit(0)
		}
	} else {
		// Read the existing JSON file
		file, err := ioutil.ReadFile(config.FileIn)
		if err != nil {
			fmt.Printf("File error: %v\n", err)
			os.Exit(1)
		}

		err = json.Unmarshal(file, &pkg)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Put the data back into a different db
	dbDest := connect(config.Destination)
	place := data.CopyQL{
		DB: dbDest,
	}

	errs := place.PutData(pkg)
	fmt.Println(errs)
}

func connect(config data.SQLConnection) *sqlx.DB {
	sourceDSN := sqlDataSource(config)
	fmt.Printf("Connecting to %s\n", sourceDSN)
	db, err := sqlx.Open("mysql", sourceDSN)
	if err != nil {
		fmt.Println(err)
		log.Fatal(err)
	}

	return db
}

func sqlDataSource(config data.SQLConnection) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s",
		config.User,
		config.Password,
		net.JoinHostPort(config.Host, config.Port),
		config.Database,
	)
}
