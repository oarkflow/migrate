package migrate

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type SeedFunction func(args ...any) (any, error)

var seedFunctions = struct {
	sync.RWMutex
	m map[string]SeedFunction
}{m: make(map[string]SeedFunction)}

func RegisterSeedFunction(name string, fn SeedFunction) {
	name = strings.TrimSpace(name)
	if name == "" || fn == nil {
		return
	}
	seedFunctions.Lock()
	defer seedFunctions.Unlock()
	seedFunctions.m[name] = fn
}

func lookupSeedFunction(name string) (SeedFunction, bool) {
	seedFunctions.RLock()
	defer seedFunctions.RUnlock()
	fn, ok := seedFunctions.m[name]
	return fn, ok
}

func randomString(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func init() {
	// Note: rand.Seed() is deprecated in Go 1.20+
	// The global random number generator is automatically seeded
	f := gofakeit.New(0)
	RegisterSeedFunction("fake_uuid", func(args ...any) (any, error) {
		return f.UUID(), nil
	})
	RegisterSeedFunction("fake_age", func(args ...any) (any, error) {
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
	RegisterSeedFunction("fake_name", func(args ...any) (any, error) {
		return f.Name(), nil
	})
	RegisterSeedFunction("fake_firstname", func(args ...any) (any, error) {
		return f.FirstName(), nil
	})
	RegisterSeedFunction("fake_lastname", func(args ...any) (any, error) {
		return f.LastName(), nil
	})
	RegisterSeedFunction("fake_email", func(args ...any) (any, error) {
		return f.Email(), nil
	})
	RegisterSeedFunction("fake_phone", func(args ...any) (any, error) {
		return f.Phone(), nil
	})
	RegisterSeedFunction("fake_address", func(args ...any) (any, error) {
		return f.Address().Address, nil
	})
	RegisterSeedFunction("fake_city", func(args ...any) (any, error) {
		return f.City(), nil
	})
	RegisterSeedFunction("fake_state", func(args ...any) (any, error) {
		return f.State(), nil
	})
	RegisterSeedFunction("fake_zip", func(args ...any) (any, error) {
		return f.Zip(), nil
	})
	RegisterSeedFunction("fake_country", func(args ...any) (any, error) {
		return f.Country(), nil
	})
	RegisterSeedFunction("fake_company", func(args ...any) (any, error) {
		return f.Company(), nil
	})
	RegisterSeedFunction("fake_jobtitle", func(args ...any) (any, error) {
		return f.JobTitle(), nil
	})
	RegisterSeedFunction("fake_ssn", func(args ...any) (any, error) {
		return f.SSN(), nil
	})
	RegisterSeedFunction("fake_creditcard", func(args ...any) (any, error) {
		return f.CreditCardNumber(nil), nil
	})
	RegisterSeedFunction("fake_currency", func(args ...any) (any, error) {
		return f.CurrencyShort(), nil
	})
	RegisterSeedFunction("fake_macaddress", func(args ...any) (any, error) {
		return f.MacAddress(), nil
	})
	RegisterSeedFunction("fake_ipv4", func(args ...any) (any, error) {
		return f.IPv4Address(), nil
	})
	RegisterSeedFunction("fake_ipv6", func(args ...any) (any, error) {
		return f.IPv6Address(), nil
	})

	RegisterSeedFunction("fake_date", func(args ...any) (any, error) {
		return f.Date(), nil
	})

	RegisterSeedFunction("fake_datetime", func(args ...any) (any, error) {
		return f.Date().Format(time.DateTime), nil
	})
	RegisterSeedFunction("fake_pastdate", func(args ...any) (any, error) {
		return f.DateRange(time.Now().AddDate(-10, 0, 0), time.Now()), nil
	})
	RegisterSeedFunction("fake_futuredate", func(args ...any) (any, error) {
		return f.DateRange(time.Now(), time.Now().AddDate(10, 0, 0)), nil
	})
	RegisterSeedFunction("fake_daterange", func(args ...any) (any, error) {
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
	RegisterSeedFunction("fake_nanosecond", func(args ...any) (any, error) {
		return f.Date().Nanosecond(), nil
	})
	RegisterSeedFunction("fake_second", func(args ...any) (any, error) {
		return f.Date().Second(), nil
	})
	RegisterSeedFunction("fake_minute", func(args ...any) (any, error) {
		return f.Date().Minute(), nil
	})
	RegisterSeedFunction("fake_hour", func(args ...any) (any, error) {
		return f.Date().Hour(), nil
	})
	RegisterSeedFunction("fake_month", func(args ...any) (any, error) {
		return int(f.Date().Month()), nil
	})
	RegisterSeedFunction("fake_monthstring", func(args ...any) (any, error) {
		return f.Date().Month().String(), nil
	})
	RegisterSeedFunction("fake_day", func(args ...any) (any, error) {
		return f.Date().Day(), nil
	})

	RegisterSeedFunction("fake_string", func(args ...any) (any, error) {
		return randomString(10), nil
	})
	RegisterSeedFunction("fake_status", func(args ...any) (any, error) {
		return f.RandomString([]string{"ACTIVE", "INACTIVE", "BANNED", "SUSPENDED"}), nil
	})
	RegisterSeedFunction("fake_bool", func(args ...any) (any, error) {
		return f.Bool(), nil
	})
	RegisterSeedFunction("fake_int", func(args ...any) (any, error) {
		return f.Int8(), nil
	})
	RegisterSeedFunction("fake_uint", func(args ...any) (any, error) {
		return f.Uint8(), nil
	})
	RegisterSeedFunction("fake_float32", func(args ...any) (any, error) {
		return f.Float32(), nil
	})
	RegisterSeedFunction("fake_float64", func(args ...any) (any, error) {
		return f.Float64(), nil
	})
	RegisterSeedFunction("fake_year", func(args ...any) (any, error) {
		return f.Date().Year(), nil
	})
	RegisterSeedFunction("fake_timezone", func(args ...any) (any, error) {
		return f.Date().Location().String(), nil
	})
	RegisterSeedFunction("fake_timezoneabv", func(args ...any) (any, error) {
		t := f.Date()
		abbr, _ := t.Zone()
		return abbr, nil
	})
	RegisterSeedFunction("fake_timezonefull", func(args ...any) (any, error) {
		return f.Date().Location().String(), nil
	})
	RegisterSeedFunction("fake_timezoneoffset", func(args ...any) (any, error) {
		t := f.Date()
		_, offset := t.Zone()
		hOffset := float32(offset) / 3600.0
		return hOffset, nil
	})
	RegisterSeedFunction("fake_timezoneregion", func(args ...any) (any, error) {
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
