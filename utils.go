package migrate

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/oarkflow/bcl"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
	f := gofakeit.New(0)
	bcl.RegisterFunction("fake_uuid", func(args ...any) (any, error) {
		return f.UUID(), nil
	})
	bcl.RegisterFunction("fake_age", func(args ...any) (any, error) {
		min := 1
		max := 100
		var ok1, ok2 bool
		if len(args) == 2 {
			min, ok1 = args[0].(int)
			max, ok2 = args[1].(int)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("fake_age arguments must be integers")
			}
		}

		if min < 0 || max < 0 || min >= max {
			return nil, fmt.Errorf("fake_age requires valid min and max values where min < max")
		}
		return rand.Intn(max-min+1) + min, nil
	})
	bcl.RegisterFunction("fake_name", func(args ...any) (any, error) {
		return f.Name(), nil
	})
	bcl.RegisterFunction("fake_firstname", func(args ...any) (any, error) {
		return f.FirstName(), nil
	})
	bcl.RegisterFunction("fake_lastname", func(args ...any) (any, error) {
		return f.LastName(), nil
	})
	bcl.RegisterFunction("fake_email", func(args ...any) (any, error) {
		return f.Email(), nil
	})
	bcl.RegisterFunction("fake_phone", func(args ...any) (any, error) {
		return f.Phone(), nil
	})
	bcl.RegisterFunction("fake_address", func(args ...any) (any, error) {
		return f.Address().Address, nil
	})
	bcl.RegisterFunction("fake_city", func(args ...any) (any, error) {
		return f.City(), nil
	})
	bcl.RegisterFunction("fake_state", func(args ...any) (any, error) {
		return f.State(), nil
	})
	bcl.RegisterFunction("fake_zip", func(args ...any) (any, error) {
		return f.Zip(), nil
	})
	bcl.RegisterFunction("fake_country", func(args ...any) (any, error) {
		return f.Country(), nil
	})
	bcl.RegisterFunction("fake_company", func(args ...any) (any, error) {
		return f.Company(), nil
	})
	bcl.RegisterFunction("fake_jobtitle", func(args ...any) (any, error) {
		return f.JobTitle(), nil
	})
	bcl.RegisterFunction("fake_ssn", func(args ...any) (any, error) {
		return f.SSN(), nil
	})
	bcl.RegisterFunction("fake_creditcard", func(args ...any) (any, error) {
		return f.CreditCardNumber(nil), nil
	})
	bcl.RegisterFunction("fake_currency", func(args ...any) (any, error) {
		return f.CurrencyShort(), nil
	})
	bcl.RegisterFunction("fake_macaddress", func(args ...any) (any, error) {
		return f.MacAddress(), nil
	})
	bcl.RegisterFunction("fake_ipv4", func(args ...any) (any, error) {
		return f.IPv4Address(), nil
	})
	bcl.RegisterFunction("fake_ipv6", func(args ...any) (any, error) {
		return f.IPv6Address(), nil
	})

	bcl.RegisterFunction("fake_date", func(args ...any) (any, error) {
		return f.Date(), nil
	})

	bcl.RegisterFunction("fake_datetime", func(args ...any) (any, error) {
		return f.Date().Format(time.DateTime), nil
	})
	bcl.RegisterFunction("fake_pastdate", func(args ...any) (any, error) {
		return f.DateRange(time.Now().AddDate(-10, 0, 0), time.Now()), nil
	})
	bcl.RegisterFunction("fake_futuredate", func(args ...any) (any, error) {
		return f.DateRange(time.Now(), time.Now().AddDate(10, 0, 0)), nil
	})
	bcl.RegisterFunction("fake_daterange", func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("fake_daterange requires 2 arguments: start and end time (YYYY-MM-DD)")
		}
		startStr, ok1 := args[0].(string)
		endStr, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("fake_daterange arguments must be strings in format YYYY-MM-DD")
		}
		start, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			return nil, err
		}
		end, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			return nil, err
		}
		return f.DateRange(start, end), nil
	})
	bcl.RegisterFunction("fake_nanosecond", func(args ...any) (any, error) {
		return f.Date().Nanosecond(), nil
	})
	bcl.RegisterFunction("fake_second", func(args ...any) (any, error) {
		return f.Date().Second(), nil
	})
	bcl.RegisterFunction("fake_minute", func(args ...any) (any, error) {
		return f.Date().Minute(), nil
	})
	bcl.RegisterFunction("fake_hour", func(args ...any) (any, error) {
		return f.Date().Hour(), nil
	})
	bcl.RegisterFunction("fake_month", func(args ...any) (any, error) {
		return int(f.Date().Month()), nil
	})
	bcl.RegisterFunction("fake_monthstring", func(args ...any) (any, error) {
		return f.Date().Month().String(), nil
	})
	bcl.RegisterFunction("fake_day", func(args ...any) (any, error) {
		return f.Date().Day(), nil
	})

	bcl.RegisterFunction("fake_string", func(args ...any) (any, error) {
		return randomString(10), nil
	})
	bcl.RegisterFunction("fake_status", func(args ...any) (any, error) {
		return f.RandomString([]string{"ACTIVE", "INACTIVE", "BANNED", "SUSPENDED"}), nil
	})
	bcl.RegisterFunction("fake_day", func(args ...any) (any, error) {
		return f.Date().Day(), nil
	})
	bcl.RegisterFunction("fake_bool", func(args ...any) (any, error) {
		return f.Bool(), nil
	})
	bcl.RegisterFunction("fake_int", func(args ...any) (any, error) {
		return f.Int8(), nil
	})
	bcl.RegisterFunction("fake_uint", func(args ...any) (any, error) {
		return f.Uint8(), nil
	})
	bcl.RegisterFunction("fake_float32", func(args ...any) (any, error) {
		return f.Float32(), nil
	})
	bcl.RegisterFunction("fake_float64", func(args ...any) (any, error) {
		return f.Float64(), nil
	})
	bcl.RegisterFunction("fake_year", func(args ...any) (any, error) {
		return f.Date().Year(), nil
	})
	bcl.RegisterFunction("fake_timezone", func(args ...any) (any, error) {
		return f.Date().Location().String(), nil
	})
	bcl.RegisterFunction("fake_timezoneabv", func(args ...any) (any, error) {
		t := f.Date()
		abbr, _ := t.Zone()
		return abbr, nil
	})
	bcl.RegisterFunction("fake_timezonefull", func(args ...any) (any, error) {
		return f.Date().Location().String(), nil
	})
	bcl.RegisterFunction("fake_timezoneoffset", func(args ...any) (any, error) {
		t := f.Date()
		_, offset := t.Zone()
		hOffset := float32(offset) / 3600.0
		return hOffset, nil
	})
	bcl.RegisterFunction("fake_timezoneregion", func(args ...any) (any, error) {
		return f.Date().Location().String(), nil
	})
}

var mysqlDataTypes = map[string]string{
	"string":     "VARCHAR",
	"varchar":    "VARCHAR",
	"text":       "TEXT",
	"char":       "CHAR",
	"longtext":   "LONGTEXT",
	"mediumtext": "MEDIUMTEXT",
	"tinytext":   "TINYTEXT",
	"number":     "INT",
	"int":        "INT",
	"integer":    "INT",
	"serial":     "INTEGER",
	"bigserial":  "BIGINT",
	"smallint":   "SMALLINT",
	"mediumint":  "MEDIUMINT",
	"bigint":     "BIGINT",
	"tinyint":    "TINYINT",
	"float":      "FLOAT",
	"double":     "DOUBLE",
	"decimal":    "DECIMAL",
	"numeric":    "DECIMAL",
	"real":       "DOUBLE",
	"boolean":    "TINYINT(1)",
	"bool":       "TINYINT(1)",
	"date":       "DATE",
	"datetime":   "DATETIME",
	"time":       "TIME",
	"timestamp":  "TIMESTAMP",
	"year":       "YEAR",
	"blob":       "BLOB",
	"mediumblob": "MEDIUMBLOB",
	"longblob":   "LONGBLOB",
	"binary":     "BLOB",
	"varbinary":  "VARBINARY",
	"enum":       "ENUM",
	"set":        "SET",
	"json":       "JSON",
	"bytea":      "BLOB",
	"bit":        "BIT",
}

var postgresDataTypes = map[string]string{
	"serial":     "SERIAL",
	"bigserial":  "BIGSERIAL",
	"string":     "TEXT",
	"varchar":    "VARCHAR",
	"text":       "TEXT",
	"char":       "CHAR",
	"longtext":   "TEXT",
	"mediumtext": "TEXT",
	"tinytext":   "TEXT",
	"shorttext":  "TEXT",
	"number":     "INTEGER",
	"int":        "INTEGER",
	"integer":    "INTEGER",
	"smallint":   "SMALLINT",
	"mediumint":  "INTEGER",
	"bigint":     "BIGINT",
	"tinyint":    "SMALLINT",
	"float":      "REAL",
	"double":     "DOUBLE PRECISION",
	"decimal":    "DECIMAL",
	"numeric":    "NUMERIC",
	"real":       "REAL",
	"boolean":    "BOOLEAN",
	"bool":       "BOOLEAN",
	"date":       "DATE",
	"datetime":   "TIMESTAMP",
	"time":       "TIME",
	"timestamp":  "TIMESTAMP",
	"year":       "INTEGER",
	"blob":       "BYTEA",
	"mediumblob": "BYTEA",
	"longblob":   "BYTEA",
	"binary":     "BYTEA",
	"varbinary":  "BYTEA",
	"bytea":      "BYTEA",
	"enum":       "TEXT",
	"set":        "TEXT",
	"json":       "JSON",
	"jsonb":      "JSONB",
	"bit":        "BIT",
}

var sqliteDataTypes = map[string]string{
	"string":     "TEXT",
	"varchar":    "VARCHAR",
	"text":       "TEXT",
	"char":       "CHAR",
	"longtext":   "TEXT",
	"mediumtext": "TEXT",
	"tinytext":   "TEXT",
	"number":     "INTEGER",
	"serial":     "INTEGER",
	"bigserial":  "INTEGER",
	"int":        "INTEGER",
	"integer":    "INTEGER",
	"smallint":   "INTEGER",
	"mediumint":  "INTEGER",
	"bigint":     "INTEGER",
	"tinyint":    "INTEGER",
	"float":      "REAL",
	"double":     "REAL",
	"decimal":    "NUMERIC",
	"numeric":    "NUMERIC",
	"real":       "REAL",
	"boolean":    "BOOLEAN",
	"bool":       "BOOLEAN",
	"date":       "DATE",
	"datetime":   "DATETIME",
	"time":       "TIME",
	"timestamp":  "DATETIME",
	"year":       "INTEGER",
	"blob":       "BLOB",
	"mediumblob": "BLOB",
	"longblob":   "BLOB",
	"binary":     "BLOB",
	"varbinary":  "BLOB",
	"bytea":      "BLOB",
	"enum":       "TEXT",
	"set":        "TEXT",
	"json":       "TEXT",
	"bit":        "NUMERIC",
}

func ConvertType(dataType string, targetDriver string, length, scale int, autoIncrement bool) string {
	if scale == 0 {
		scale = 2
	}
	lt := strings.ToLower(dataType)
	if length > 0 && scale > length {
		scale = length
	}
	var dt string
	var ok bool
	switch targetDriver {
	case "mysql":
		dt, ok = mysqlDataTypes[lt]
	case "postgres":
		dt, ok = postgresDataTypes[lt]
	case "sqlite":
		dt, ok = sqliteDataTypes[lt]
	default:
		return lt
	}
	if !ok {
		return lt
	}
	if autoIncrement && targetDriver == "postgres" {
		switch lt {
		case "int", "integer", "number", "smallint", "mediumint":
			if length > 10 {
				return "BIGSERIAL"
			}
			return "SERIAL"
		case "bigint":
			return "BIGSERIAL"
		}
	}
	switch lt {
	case "varchar", "char", "bit":
		if length > 0 {
			return fmt.Sprintf("%s(%d)", dt, length)
		}
		if lt == "varchar" || lt == "char" {
			return fmt.Sprintf("%s(255)", dt)
		}
	case "string":
		if length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", length)
		}
		return "VARCHAR(255)"
	case "decimal", "numeric":
		if length > 0 {
			if scale > 0 {
				return fmt.Sprintf("%s(%d,%d)", dt, length, scale)
			}
			return fmt.Sprintf("%s(%d,2)", dt, length)
		}
	}
	return dt
}
