package sqldb

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lexacali/fivethreeone/fto"
)

type sqlWeight struct {
	w *fto.Weight
}

func (sw sqlWeight) Value() (driver.Value, error) {
	return fmt.Sprintf("%d:%s", sw.w.Value, sw.w.Unit), nil
}

func (sw *sqlWeight) Scan(val interface{}) error {
	if val == nil {
		return errors.New("weight should always be set")
	}

	switch v := val.(type) {
	case []byte:
		w, err := parseWeight(v)
		if err != nil {
			return fmt.Errorf("failed to parse weight: %w", err)
		}
		*sw.w = w
		return nil
	default:
		return fmt.Errorf("unexpected type of val %T", val)
	}
}

func parseWeight(v []byte) (fto.Weight, error) {
	ps := strings.Split(string(v), ":")
	if n := len(ps); n != 2 {
		return fto.Weight{}, fmt.Errorf("malformed weight had %d parts", n)
	}

	val, err := strconv.Atoi(ps[0])
	if err != nil {
		return fto.Weight{}, fmt.Errorf("failed to parse weight %q: %w", ps[0], err)
	}

	var unit fto.WeightUnit
	switch ps[1] {
	case "DECI_POUNDS":
		unit = fto.DeciPounds
	default:
		return fto.Weight{}, fmt.Errorf("unknown unit %q", ps[1])
	}

	return fto.Weight{
		Unit:  unit,
		Value: val,
	}, nil
}

func nullString(in string) sql.NullString {
	if in == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{Valid: true, String: in}
}
