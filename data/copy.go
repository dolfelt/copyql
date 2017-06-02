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
	DB *sqlx.DB
	// relations Relations
}

// PointerValue hold the value to update in a column
type PointerValue struct {
	Pointer
	Value interface{}
}

// GetData gathers all data relating to an entry point
func (c *CopyQL) GetData(entry Pointer, id interface{}, relations Relations) TableData {
	copy := &copyData{
		CopyQL:    c,
		relations: relations,
		completed: map[string]bool{},
		channel:   make(chan PointerValue),
		data:      TableData{},
	}

	copy.cachedPlan = copy.plan(entry)
	return copy.traverse(entry, id, true)
}

// PutData loads data from JSON into a sql
func (c *CopyQL) PutData(data TableData) []error {
	errs := []error{}

	for table, rows := range data {
		fmt.Printf("Inserting %s\n", table)
		err := c.putTableData(table, rows)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

type copyData struct {
	*CopyQL
	relations  Relations
	cachedPlan map[string]string
	completed  map[string]bool
	channel    chan PointerValue
	data       TableData
}

func (c *copyData) plan(entry Pointer) map[string]string {
	plan := map[string]string{}
	if sub, ok := c.relations[entry.Table]; ok {
		for _, rel := range sub {
			if rel.ForeignKey != "id" {
				continue
			}
			plan[rel.Table] = rel.Column
		}

		for _, rel := range sub {
			if rel.ForeignKey != "id" {
				continue
			}
			subPlan := c.plan(rel.Pointer)
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
func (c *copyData) traverse(entry Pointer, value interface{}, deep bool) TableData {
	data := TableData{}
	valueKey := fmt.Sprintf("%s:%s", entry.String(), value)

	// Skip because this relation has already been queried
	if done, ok := c.completed[entry.String()]; ok && done {
		fmt.Printf("Skipping %s\n", entry.String())
		return data
	}
	if done, ok := c.completed[valueKey]; ok && done {
		fmt.Printf("Skipping %s for value %s\n", entry.String(), value)
		return data
	}
	if primary, ok := c.cachedPlan[entry.Table]; ok && primary != entry.Column {
		fmt.Printf("Skipping because %s is not primary: %s\n", entry.String(), primary)
		return data
	}

	rows, err := c.getTableData(entry.Table, entry.Column, value)
	data[entry.Table] = rows
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
				tr := c.traverse(rel.Pointer, id, rel.ForeignKey == "id")
				for t, r := range tr {
					data[t] = append(data[t], r...)
				}
			}
		}
	}

	return data
}

func (c *copyData) getTableData(table string, column string, id interface{}) ([]tableRow, error) {
	fmt.Printf("Selecting %s.%s from %d\n", table, column, id)
	var res []tableRow
	rows, err := c.DB.Queryx(fmt.Sprintf("SELECT * FROM %s WHERE %s=?", table, column), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rels := relationMap(c.relations[table])

	for rows.Next() {
		row := tableRow{}
		rows.MapScan(row)
		convertStrings(row, rels)
		res = append(res, row)
	}

	return res, nil
}

func relationMap(relations []Relation) map[string]Relation {
	out := map[string]Relation{}

	for _, rel := range relations {
		out[rel.Column] = rel
	}

	return out
}

func (c *CopyQL) putTableData(table string, rows []tableRow) error {

	temp := "INSERT INTO %s (`%s`) VALUES (%s)"

	tx, err := c.DB.Beginx()
	if err != nil {
		return err
	}
	for _, row := range rows {
		cols := []string{}
		binds := []string{}
		vals := []interface{}{}
		for c, v := range row {
			cols = append(cols, c)
			binds = append(binds, "?")
			// if v == nil {
			// 	v = ""
			// }
			vals = append(vals, v)
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

func convertStrings(in map[string]interface{}, relations map[string]Relation) {
	for k, v := range in {
		t := reflect.TypeOf(v)
		if t != nil {
			switch t.Kind() {
			case reflect.Slice:
				in[k] = fmt.Sprintf("%s", v)

			default:
				// do nothing
			}
		} else {
			if rel, ok := relations[k]; ok {
				if !rel.Nullable {
					in[k] = ""
				}
			}
		}
	}
}
