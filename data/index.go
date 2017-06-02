package data

import (
	"fmt"
	"strings"

	"github.com/gedex/inflector"
)

// Pointer describes half of a relationship
type Pointer struct {
	Table  string
	Column string
}

// Column holds information about an individual column
type Column struct {
	Name     string
	Nullable bool
}

// Relation holds information about a relationship
type Relation struct {
	Pointer
	ForeignKey string
	Nullable   bool
}

// Relations maps the source table and column with the destination
type Relations map[string][]Relation
type itemList map[string]struct{}

func (p *Pointer) String() string {
	return fmt.Sprintf("%s.%s", p.Table, p.Column)
}

// BuildRelations builds relations from the source table
func (c *CopyQL) BuildRelations() (*Relations, error) {
	tables, err := c.getTables()
	if err != nil {
		return nil, err
	}

	rel := Relations{}

	for _, t := range tables {
		columns, err := c.getIDColumns(t)
		if err != nil {
			return nil, err
		}
		fmt.Println(t)

		for _, c := range columns {
			term := strings.TrimSuffix(c.Name, "_id")
			linkTable := inflector.Pluralize(term)
			if !tableExists(tables, linkTable) {
				continue
			}
			rel[linkTable] = append(rel[linkTable], Relation{
				Pointer{Table: t, Column: c.Name},
				"id",
				c.Nullable,
			})
			rel[t] = append(rel[t], Relation{
				Pointer{Table: linkTable, Column: "id"},
				c.Name,
				false,
			})
		}
	}
	return &rel, nil
}

func tableExists(tables []string, term string) bool {
	for _, t := range tables {
		if t == term {
			return true
		}
	}
	return false
}

func (c *CopyQL) getIDColumns(table string) ([]Column, error) {
	columns, err := c.getColumns(table)
	if err != nil {
		return nil, err
	}

	ids := []Column{}
	for _, col := range columns {
		if strings.HasSuffix(col.Name, "_id") {
			ids = append(ids, col)
		}
	}

	return ids, nil
}

func (c *CopyQL) getColumns(table string) ([]Column, error) {
	rows, err := c.DB.Query(fmt.Sprintf("SHOW COLUMNS FROM %s", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := []Column{}
	for rows.Next() {
		var colName string
		var colNullable string
		rows.Scan(&colName, nil, &colNullable, nil, nil, nil)
		columns = append(columns, Column{
			Name:     colName,
			Nullable: colNullable == "YES",
		})
	}

	return columns, nil
}

func (c *CopyQL) getTables() ([]string, error) {
	rows, err := c.DB.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []string{}
	for rows.Next() {
		var table string
		rows.Scan(&table)
		tables = append(tables, table)
	}

	return tables, nil
}
