package basemodel

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

type (
	BaseModel struct {
		ID          uint64    `json:"id" sql:"AUTO_INCREMENT" gorm:"primary_key,column:id"`
		CreatedTime time.Time `json:"created_time" gorm:"column:created_time" sql:"DEFAULT:current_timestamp"`
		UpdatedTime time.Time `json:"updated_time" gorm:"column:updated_time" sql:"DEFAULT:current_timestamp"`
	}

	DBFunc func(tx *gorm.DB) error

	PagedSearchResult struct {
		TotalData   int         `json:"total_data"`   // matched datas
		Rows        int         `json:"rows"`         // shown datas per page
		CurrentPage int         `json:"current_page"` // current page
		LastPage    int         `json:"last_page"`
		From        int         `json:"from"` // offset, starting index of data shown in current page
		To          int         `json:"to"`   // last index of data shown in current page
		Data        interface{} `json:"data"`
	}

	CompareFilter struct {
		Value1 interface{} `json:"value1"`
		Value2 interface{} `json:"value2"`
	}
)

var db *gorm.DB

// helper for inserting data using gorm.DB functions
func WithinTransaction(fn DBFunc) (err error) {
	tx := db.Begin()
	defer tx.Commit()
	err = fn(tx)

	return err
}

// inserts a row into db.
func Create(i interface{}) error {
	return WithinTransaction(func(tx *gorm.DB) (err error) {
		if !tx.NewRecord(i) {
			return err
		}
		if err = tx.Create(i).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

// update row in db.
func Save(i interface{}) error {
	return WithinTransaction(func(tx *gorm.DB) (err error) {
		// check new object
		if tx.NewRecord(i) {
			return err
		}
		if err = tx.Save(i).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

// delete row in db.
func Delete(i interface{}) error {
	return WithinTransaction(func(tx *gorm.DB) (err error) {
		// check new object
		if err = tx.Delete(i).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

// Find by id.
func FindbyID(i interface{}, id int) error {
	return WithinTransaction(func(tx *gorm.DB) (err error) {
		if err = tx.Last(i, id).Error; err != nil {
			tx.Rollback()
			return err
		}
		return err
	})
}

func FilterSearchSingle(i interface{}, filter interface{}) (err error) {
	// filtering
	refFilter := reflect.ValueOf(filter).Elem()
	refType := refFilter.Type()
	query(&db, refType)

	if err = db.Last(i).Error; err != nil {
		db.Rollback()
	}

	return err
}

func FilterSearch(i interface{}, filter interface{}) (err error) {
	// filtering
	refFilter := reflect.ValueOf(filter).Elem()
	refType := refFilter.Type()
	query(&db, refType)

	var total_rows int
	if err = db.Find(i).Error; err != nil {
		db.Rollback()
	}

	return err
}

func PagedFilterSearch(i interface{}, page int, rows int, orderby string, sort string, filter interface{}) (result PagedSearchResult, err error) {
	if page <= 0 {
		page = 1
	}

	if rows <= 0 {
		rows = 25 // default row is 25 per page
	}

	// filtering
	refFilter := reflect.ValueOf(filter).Elem()
	refType := refFilter.Type()
	query(&db, refType)

	// ordering and sorting
	if orderby != "" {
		orders := strings.Split(orderby, ",")
		sort := strings.Split(sort, ",")

		for k, v := range orders {
			e := v
			if len(sort) > k {
				value := sort[k]
				if strings.ToUpper(value) == "ASC" || strings.ToUpper(value) == "DESC" {
					e = v + " " + strings.ToUpper(value)
				}
			}
			db = db.Order(e)
		}
	}

	tempDB := db
	var (
		total_rows int
		lastPage   int = 1 // default 1
	)

	tempDB.Find(i).Count(&total_rows)

	offset := (page * rows) - rows
	lastPage = int(math.Ceil(float64(total_rows) / float64(rows)))

	b.DB.Limit(rows).Offset(offset).Find(i)

	result = PagedSearchResult{
		TotalData:   total_rows,
		Rows:        rows,
		CurrentPage: page,
		LastPage:    lastPage,
		From:        offset + 1,
		To:          offset + rows,
		Data:        &i,
	}

	return result, err
}

func query(db *gorm.DB, t reflect.Type) {
	for x := 0; x < t.NumField(); x++ {
		field := t.Field(x)
		if field.Interface() != "" {
			switch t.Field(x).Tag.Get("condition") {
			default:
				db = db.Where(fmt.Sprintf("%s = ?", t.Field(x).Tag.Get("json")), field.Interface())
			case "LIKE":
				db = db.Where(fmt.Sprintf("LOWER(%s) %s ?", t.Field(x).Tag.Get("json"), t.Field(x).Tag.Get("condition")), "%"+strings.ToLower(field.Interface().(string))+"%")
			case "OR":
				var e []string
				for _, filter := range field.Interface().([]string) {
					e = append(e, t.Field(x).Tag.Get("json")+" = '"+filter+"' ")
				}
				db = db.Where(strings.Join(e, " OR "))
			case "BETWEEN":
				if values, ok := field.Interface().(CompareFilter); ok && values.Value1 != "" {
					db = db.Where(fmt.Sprintf("%s %s ? %s ?", t.Field(x).Tag.Get("json"), t.Field(x).Tag.Get("condition"), "AND"), values.Value1, values.Value2)
				}
			}
		}
	}
}
