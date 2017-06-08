package data

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx"
)

type tableRow map[string]interface{}

// TableData is generic table data
type TableData map[string][]tableRow

// CopyQL manages the data needed for copying
type CopyQL struct {
	DB         *sqlx.DB
	SkipTables []string
	Verbose    bool
}

type copyData struct {
	*CopyQL
	relations  Relations
	columns    Columns
	cachedPlan map[string]string
	completed  map[string]bool
	data       TableData
}

// ColumnValue hold the value to update in a column
type ColumnValue struct {
	Column
	Value interface{}
}

// GetData gathers all data relating to an entry point
func (c *CopyQL) GetData(entry ColumnValue, columns Columns, relations Relations) TableData {
	copy := &copyData{
		CopyQL:    c,
		columns:   columns,
		relations: relations,
		completed: map[string]bool{},
		data:      TableData{},
	}

	copy.cachedPlan = copy.plan(entry.Column)
	return copy.traverse(entry, true)
}

// PutData loads data from JSON into a sql
func (c *CopyQL) PutData(data TableData, columns Columns) []error {
	errs := []error{}
	copy := &copyData{
		CopyQL:  c,
		columns: columns,
	}
	for table, rows := range data {
		fmt.Printf("Inserting %s\n", table)
		err := copy.putTableData(table, rows)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (c *copyData) plan(entry Column) map[string]string {
	plan := map[string]string{}
	if sub, ok := c.relations[entry.Table]; ok {
		for _, rel := range sub {
			if rel.ForeignKey != "id" {
				continue
			}
			plan[rel.Table] = rel.Name
		}

		for _, rel := range sub {
			if rel.ForeignKey != "id" {
				continue
			}
			subPlan := c.plan(rel.Column)
			for t, c := range subPlan {
				if _, ok := plan[t]; ok {
					continue
				}
				plan[t] = c
			}
		}
	}

	return plan
}

// Traverse gathers all data relating to an entry point. It recursively traverses all relationships
// to gather all the data.
func (c *copyData) traverse(entry ColumnValue, deep bool) TableData {
	data := TableData{}
	valueKey := fmt.Sprintf("%s:%s", entry.String(), entry.Value)

	for _, skip := range c.SkipTables {
		if skip == entry.Table {
			if c.Verbose {
				fmt.Printf("Skipping table %s\n", entry.Table)
			}
			return data
		}
	}

	// Skip because this relation has already been queried
	if done, ok := c.completed[entry.String()]; ok && done {
		if c.Verbose {
			fmt.Printf("Skipping %s\n", entry.String())
		}
		return data
	}
	if done, ok := c.completed[valueKey]; ok && done {
		if c.Verbose {
			fmt.Printf("Skipping %s for value %s\n", entry.String(), entry.Value)
		}
		return data
	}
	if primary, ok := c.cachedPlan[entry.Table]; ok && primary != entry.Name {
		if c.Verbose {
			fmt.Printf("Skipping because %s is not primary: %s\n", entry.String(), primary)
		}
		return data
	}

	rows, err := c.getTableData(entry.Table, entry.Name, entry.Value)
	if len(rows) > 0 {
		data[entry.Table] = rows
	}
	if deep {
		c.completed[entry.String()] = true
	}
	c.completed[valueKey] = true

	// Return if we aren't interested in digging deeper into the relationships
	if !deep {
		return data
	}

	if err != nil {
		fmt.Println(err)
	}
	if next, ok := c.relations[entry.Table]; ok {
		for _, row := range rows {
			for _, rel := range next {
				id, ok := row[rel.ForeignKey]
				if !ok {
					continue
				}
				// Continue traversing. Deeply only if the ForeignKey is id
				tr := c.traverse(ColumnValue{rel.Column, id}, rel.ForeignKey == "id")
				for t, r := range tr {
					data[t] = append(data[t], r...)
				}
			}
		}
	}

	return data
}

func (c *copyData) getTableData(table string, column string, id interface{}) ([]tableRow, error) {
	if c.Verbose {
		fmt.Printf("Selecting %s.%s from %d\n", table, column, id)
	}
	var res []tableRow
	rows, err := c.DB.Queryx(fmt.Sprintf("SELECT * FROM %s WHERE %s=?", table, column), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := columnMap(c.columns[table])

	for rows.Next() {
		row := tableRow{}
		rows.MapScan(row)
		convertStrings(row, cols)
		res = append(res, row)
	}

	return res, nil
}

func columnMap(columns []Column) map[string]Column {
	out := map[string]Column{}

	for _, col := range columns {
		out[col.Name] = col
	}

	return out
}

func (c *copyData) putTableData(table string, rows []tableRow) error {

	temp := "INSERT INTO %s (`%s`) VALUES (%s)"

	tx, err := c.DB.Beginx()
	if err != nil {
		return err
	}

	columns, ok := c.columns[table]
	if !ok {
		return fmt.Errorf("No table information found for %s", table)
	}
	colData := columnMap(columns)

	for _, row := range rows {
		cols := []string{}
		binds := []string{}
		vals := []interface{}{}
		for c, v := range row {
			if cInfo, ok := colData[c]; ok {
				cols = append(cols, c)
				binds = append(binds, "?")
				if v == nil && !cInfo.Nullable {
					v = ""
				}
				if v == "" && cInfo.Nullable {
					v = nil
				}
				vals = append(vals, v)
			}
		}
		stmt := fmt.Sprintf(temp, table, strings.Join(cols, "`,`"), strings.Join(binds, ","))
		_, err = tx.Exec(stmt, vals...)
		if err != nil {
			fmt.Println(err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func convertStrings(in map[string]interface{}, columns map[string]Column) {
	for k, v := range in {
		t := reflect.TypeOf(v)
		var col Column
		var ok bool
		if col, ok = columns[k]; !ok {
			col = Column{Name: k}
		}
		if t != nil {
			switch t.Kind() {
			case reflect.Slice:
				in[k] = fmt.Sprintf("%s", v)
				if len(in[k].(string)) == 0 && col.Nullable {
					in[k] = nil
				}
			default:
				// do nothing
			}
		} else {
			if !col.Nullable {
				in[k] = ""
			}
		}
	}
}
