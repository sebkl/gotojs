package gotojs

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTimeConverter(t *testing.T) {
	ti := time.Now()

	s := ti.Format(time.RFC3339Nano)
	i := ti.Unix() * 1000

	tt := reflect.TypeOf(ti)

	sv, err := TimeConverter(s, tt)
	if err != nil {
		t.Errorf("%s", err)
	}
	if svt, ok := sv.(time.Time); !ok {
		t.Errorf("Time conversion failed: %s/%s", svt, ti)
	} else if svt.UnixNano() != ti.UnixNano() {
		t.Errorf("Time conversion dos not match: %d/%d", svt.UnixNano(), ti.UnixNano())
	}

	iv, err := TimeConverter(i, tt)
	if err != nil {
		t.Errorf("%s", err)
	}
	if ivt, ok := iv.(time.Time); !ok {
		t.Errorf("Time conversion failed: %s/%s", ivt, ti)
	} else if ivt.Unix() != ti.Unix() {
		t.Errorf("Time conversion dos not match: %d/%d", ivt.Unix(), ti.Unix())
	}
}

// TestTimeConverterDates checks whether full day formats are identified as dates properly.
func TestTimeConverterDates(t *testing.T) {
	/* dates */
	ti, _ := time.Parse(time.RFC3339, "2015-01-03T00:00:00Z")

	for _, da := range []string{"2015-01-03", "2015.01.03", "2015/03/01", "1420243200000"} {
		iv, err := TimeConverter(da, reflect.TypeOf(ti))
		if err != nil {
			t.Errorf("%s", err)
		}
		if ivt, ok := iv.(time.Time); !ok {
			t.Errorf("Time conversion failed: %s/%s", iv, ti)
		} else if ivt.Unix() != ti.Unix() {
			t.Errorf("Time conversion dos not match: %d/%d", ivt.Unix(), ti.Unix())
		}
	}
}

func TestStringConverter(t *testing.T) {
	f := 4545.65772
	fv, _ := StringConverter(f, reflect.TypeOf(""))
	if sfv, ok := fv.(string); ok {
		if !strings.HasPrefix(sfv, "4545.65772") {
			t.Errorf("String convertion failed: %s/%s", sfv, "4545.65772")
		}
	} else {
		t.Errorf("Could not convert String.")
	}
}
