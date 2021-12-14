package generator

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"

	_ "github.com/go-sql-driver/mysql"
)

type schemaFetcher interface {
	GetDatabaseName() (dbName string, err error)
	GetTableNames() (tableNames []string, err error)
	GetFieldDescriptors(tableName string) ([]fieldDescriptor, error)
	QuoteIdentifier(identifier string) string
	GetCreateSyntax(tableName string) (createSyntax string, err error)
}

// Generate generates code for the given driverName.
func Generate(driverName string, dsn string, tableNames []string, path string) error {
	//dataSourceName, tableNames := parseArgs(exampleDataSourceName)
	if dsn == "" {
		return fmt.Errorf("Notice: dsn not configured,schema may have difference")
	}
	fmt.Println(driverName, dsn)
	db, err := sql.Open(driverName, dsn)
	defer db.Close()
	if err != nil {
		return err
	}

	schemaFetcherFactory := getSchemaFetcherFactory(driverName)
	schemaFetcher := schemaFetcherFactory(db)

	dbName, err := schemaFetcher.GetDatabaseName()
	if err != nil {
		return err
	}

	if dbName == "" {
		return errors.New("no database selected")
	}

	if len(tableNames) == 0 {
		tableNames, err = schemaFetcher.GetTableNames()
		if err != nil {
			return err
		}
	}
	for _, tableName := range tableNames {
		createSyntax, err := schemaFetcher.GetCreateSyntax(tableName)
		if err != nil {
			return err
		}
		if path == "" {
			return fmt.Errorf("path is empty")
		}
		if path[len(path)-1] != '/' {
			path += "/"
		}
		fname := path + tableName + ".sql"
		file, er := os.OpenFile(fname, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0766)
		defer file.Close()
		if er != nil && os.IsNotExist(er) {
			if file, err = os.Create(fname); err != nil {
				return err
			}
		}
		reg := regexp.MustCompile("AUTO_INCREMENT=[0-9]* ")
		if reg.Match([]byte(createSyntax)) {
			createSyntax = string(reg.ReplaceAll([]byte(createSyntax), []byte(" ")))
		}
		file.WriteString(createSyntax + ";")
		fmt.Println("generated:", fname)
	}

	return nil
}

func getSchemaFetcherFactory(driverName string) func(db *sql.DB) schemaFetcher {
	switch driverName {
	case "mysql":
		return newMySQLSchemaFetcher
	default:
		_, _ = fmt.Fprintln(os.Stderr, "unsupported driver "+driverName)
		os.Exit(2)
		return nil
	}
}
