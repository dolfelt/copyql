package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"github.com/dolfelt/copyql/data"
	"github.com/fatih/color"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configFile string

func init() {
	RootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default is ./copyql.yaml)")

	RootCmd.PersistentFlags().StringSlice("skip", []string{}, "a comma separated list of tables to skip copying")

	RootCmd.PersistentFlags().String("in", "", "json file to read into the destination data store")
	viper.BindPFlag("FileIn", RootCmd.PersistentFlags().Lookup("in"))

	RootCmd.PersistentFlags().String("out", "", "json file to write the data from the source into")
	viper.BindPFlag("FileOut", RootCmd.PersistentFlags().Lookup("out"))

	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "print debugging information")
	viper.BindPFlag("Verbose", RootCmd.PersistentFlags().Lookup("verbose"))

}

// RootCmd controls the entry into the application
var RootCmd = &cobra.Command{
	Use:   "copyql <table>.<column>:<value>",
	Short: "Copy data and relationships from one SQL store to another",
	Run:   rootRun,
}

func rootRun(cmd *cobra.Command, args []string) {
	config, err := data.LoadConfig(configFile)
	if err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}

	skip, err := cmd.PersistentFlags().GetStringSlice("skip")
	if err != nil {
		color.Red(err.Error())
		os.Exit(1)
	}
	config.SkipTables = append(config.SkipTables, skip...)

	var pkg data.TableData

	if len(config.FileIn) == 0 {
		if len(args) != 1 {
			color.Red("Please include an entry point.")
			os.Exit(1)
		}

		entryColumn, err := getEntryColumn(args[0])
		if err != nil {
			color.Red(err.Error())
			os.Exit(1)
		}

		dbSource := connect(config.Source)
		defer dbSource.Close()

		if len(config.SkipTables) > 0 {
			fmt.Printf("Skipping tables %s\n", strings.Join(config.SkipTables, ", "))
		}

		fmt.Println("Copying data...")

		copy := data.CopyQL{
			DB:         dbSource,
			SkipTables: config.SkipTables,
			Verbose:    config.Verbose,
		}

		columns, relations, err := copy.AnalyzeDatabase()
		if err != nil {
			color.Red("Error generating automatic relations in the source. %s", err)
			os.Exit(1)
		}

		relations, err = copy.ParseCustomRelations(config.Relations, columns, relations)
		if err != nil {
			color.Red("Error use custom relations. %s", err)
			os.Exit(1)
		}

		pkg = copy.GetData(*entryColumn, *columns, *relations)

		if len(pkg) == 0 {
			color.Yellow("No data found")
			os.Exit(1)
		}

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
			fmt.Printf("Writing data to %s\n", config.FileOut)
			os.Exit(0)
		}
	} else {
		fmt.Printf("Reading data from file: %s\n", config.FileIn)
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
		DB:      dbDest,
		Verbose: config.Verbose,
	}

	columns, _, err := place.AnalyzeDatabase()
	if err != nil {
		color.Red("Error generating automatic relations in the destination. %s", err)
		os.Exit(1)
	}

	errs := place.PutData(pkg, *columns)

	if len(errs) > 0 {
		fmt.Println(errs)
		color.Yellow("Completed with errors!")
	} else {
		color.Green("OK")
	}
}

func getEntryColumn(entry string) (*data.ColumnValue, error) {
	formatError := "Please include a valid entry point: <table>.<column>:<value>"
	entryParts := strings.SplitN(entry, ":", 2)
	if len(entryParts) != 2 {
		return nil, errors.New(formatError)
	}
	entryColumn, err := data.ColumnFromString(entryParts[0])
	if err != nil {
		return nil, errors.New(formatError)
	}

	return &data.ColumnValue{
		Column: entryColumn,
		Value:  entryParts[1],
	}, nil
}

func connect(config data.SQLConnection) *sqlx.DB {
	sourceDSN := sqlDataSource(config)
	fmt.Printf("Connecting to %s\n", sourceDSN)
	db, err := sqlx.Open("mysql", sourceDSN)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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
