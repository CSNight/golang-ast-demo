package db

import (
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

type Query struct {
	Field string      `json:"field"`
	Type  string      `json:"type"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

var operators = map[string]string{
	"l_like":    " LIKE ",
	"r_like":    " LIKE ",
	"in_like":   " LIKE ",
	"gte":       " >= ",
	"gt":        " > ",
	"lte":       " < ",
	"lt":        " <= ",
	"equal":     " = ",
	"not_equal": " <> ",
	"in":        " IN ",
}

func parseFilter(filter interface{}) []Query {
	var filters []Query
	tp := reflect.TypeOf(filter)
	vars := reflect.ValueOf(filter)
	for i := 0; i < tp.NumField(); i++ {
		tag := tp.Field(i).Tag
		queryTag := tag.Get("query")
		if queryTag != "" {
			parts := strings.Split(strings.TrimSpace(queryTag), ",")
			if len(parts) >= 2 {
				typePart := strings.Split(parts[0], ":")
				fieldPart := strings.Split(parts[1], ":")
				if len(typePart) == 2 && len(fieldPart) == 2 {
					if op, ok := operators[typePart[1]]; ok {
						ft := vars.Field(i).Type().Name()
						if vars.Field(i).IsZero() && ft != "bool" &&
							len(parts) > 2 && parts[2] == "omitempty" {
							continue
						}
						query := Query{
							Field: fieldPart[1],
							Type:  typePart[1],
							Op:    op,
							Value: vars.Field(i).Interface(),
						}
						filters = append(filters, query)
					}
				}
			}
		}
	}
	return filters
}

func BuildWhere(db *gorm.DB, filter interface{}) *gorm.DB {
	if filter == nil {
		return db
	}
	filters := parseFilter(filter)
	for _, f := range filters {
		if f.Value == nil {
			continue
		}
		switch f.Type {
		case "in_like":
			if strings.Contains(f.Field, "|") {
				fields := strings.Split(f.Field, "|")
				for _, fd := range fields {
					db = db.Or(fd+f.Op+"?", "%"+fmt.Sprintf("%v", f.Value)+"%")
				}
				continue
			}
			db = db.Where(f.Field+f.Op+"?", "%"+fmt.Sprintf("%v", f.Value)+"%")
		case "r_like":
			db = db.Where(f.Field+f.Op+"?", fmt.Sprintf("%v", f.Value)+"%")
		case "l_like":
			db = db.Where(f.Field+f.Op+"?", "%"+fmt.Sprintf("%v", f.Value))
		case "in", "not_equal", "equal", "lt", "lte", "gt", "gte":
			db = db.Where(f.Field+f.Op+"?", f.Value)
		}
	}
	return db
}
