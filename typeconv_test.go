package gotojs

import (
	"testing"
	"time"
	"reflect"
	"strings"
)

func TestTimeConverter(t *testing.T) {
	ti := time.Now()

	s := ti.Format(time.RFC3339Nano)
	i := ti.Unix() *1000

	tt := reflect.TypeOf(ti)

	sv,err := TimeConverter(s,tt)
	if err != nil {
		t.Errorf("%s",err)
	}
	if svt,ok := sv.(time.Time); !ok {
		t.Errorf("Time conversion failed: %s/%s",sv,ti)
	} else if svt.UnixNano() != ti.UnixNano(){
		t.Errorf("Time conversion dos not match: %d/%d",svt.UnixNano(),ti.UnixNano())
	}


	iv,err := TimeConverter(i,tt)
	if err != nil {
		t.Errorf("%s",err)
	}
	if ivt,ok := iv.(time.Time); !ok {
		t.Errorf("Time conversion failed: %s/%s",ivt,ti)
	} else if ivt.Unix() != ti.Unix(){
		t.Errorf("Time conversion dos not match: %d/%d",ivt.Unix(),ti.Unix())
	}
}

func TestStringConverter(t *testing.T) {
	f := 4545.65772
	fv,_ := StringConverter(f,reflect.TypeOf(""))
	if sfv, ok:= fv.(string); ok {
		if !strings.HasPrefix(sfv,"4545.65772") {
			t.Errorf("String convertion failed: %s/%s",sfv,"4545.65772")
		}
	} else {
		t.Errorf("Could not convert String.")
	}
}
