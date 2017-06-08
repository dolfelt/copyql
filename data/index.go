package data

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gedex/inflector"
)

// Column holds information about an individual column
type Column struct {
	Table    string
	Name     string
	Nullable bool
}

// Relation holds information about a relationship
type Relation struct {
	Column
	ForeignKey string
}

// Relations maps the source table and column with the destination
type Relations map[string][]Relation

// Columns maps the table to its list of columns
type Columns map[string][]Column

type itemList map[string]struct{}

func (p *Column) String() string {
	return fmt.Sprintf("%s.%s", p.Table, p.Name)
}

// ColumnFromString parses a string into a Pointer
func ColumnFromString(column string) (Column, error) {
	parts := strings.SplitN(column, ".", 2)
	if len(parts) != 2 {
		return Column{}, fmt.Errorf("Invalid column string %s", column)
	}

	return Column{
		Table: parts[0],
		Name:  parts[1],
	}, nil
}

// RelationshipFromColumns creates a relationship from a Pointer and a Column
func RelationshipFromColumns(from Column, to Column) Relation {
	return Relation{
		Column:     from,
		ForeignKey: to.Name,
	}
}

// AnalyzeDatabase reads column information and build relations from the database
func (c *CopyQL) AnalyzeDatabase() (*Columns, *Relations, error) {
	tables, err := c.getTables()
	if err != nil {
		return nil, nil, err
	}

	rel := Relations{}
	cols := Columns{}

	for _, t := range tables {
		columns, err := c.getColumns(t)
		if err != nil {
			return nil, nil, err
		}

		if c.Verbose {
			fmt.Printf("Getting relations for table: %s\n", t)
		}

		for _, c := range columns {
			cols[t] = append(cols[t], c)
			if !strings.HasSuffix(c.Name, "_id") {
				continue
			}
			term := strings.TrimSuffix(c.Name, "_id")
			linkTable := inflector.Pluralize(term)
			if !tableExists(tables, linkTable) {
				continue
			}

			destCol := Column{
				Table: linkTable,
				Name:  "id",
			}

			rel = addRelation(rel, c, destCol)
		}
	}
	return &cols, &rel, nil
}

// ParseCustomRelations parses custom relations as they relate to the table
func (c *CopyQL) ParseCustomRelations(custom map[string]string, columns *Columns, relations *Relations) (*Relations, error) {
	rel := *relations
	for s, d := range custom {
		src, err := ColumnFromString(s)
		if err != nil {
			return relations, err
		}
		dest, err := ColumnFromString(d)
		if err != nil {
			return relations, err
		}

		src, err = fillColumn(src, *columns)
		if err != nil {
			return relations, err
		}
		dest, err = fillColumn(dest, *columns)
		if err != nil {
			return relations, err
		}

		rel = addRelation(rel, src, dest)
	}
	return &rel, nil
}

func addRelation(rel Relations, src Column, dest Column) Relations {
	rel[dest.Table] = append(rel[dest.Table], Relation{
		Column:     src,
		ForeignKey: dest.Name,
	})
	rel[src.Table] = append(rel[src.Table], Relation{
		Column:     dest,
		ForeignKey: src.Name,
	})

	return rel
}

func fillColumn(col Column, columns Columns) (Column, error) {
	if cols, ok := columns[col.Table]; ok {
		cmap := columnMap(cols)
		if c, ok := cmap[col.Name]; ok {
			return c, nil
		}
	}
	return Column{}, fmt.Errorf("Cannot find column in database: %s", col.String())
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
	rows, err := c.DB.Queryx(fmt.Sprintf("SHOW COLUMNS FROM %s", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type columnInfo struct {
		Field   string         `db:"Field"`
		Type    string         `db:"Type"`
		Null    string         `db:"Null"`
		Key     string         `db:"Key"`
		Default sql.NullString `db:"Default"`
		Extra   string         `db:"Extra"`
	}

	columns := []Column{}
	for rows.Next() {
		var cols columnInfo
		rows.StructScan(&cols)
		columns = append(columns, Column{
			Table:    table,
			Name:     cols.Field,
			Nullable: cols.Null == "YES",
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
