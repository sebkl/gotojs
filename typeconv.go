package gotojs

import (
	"time"
	"fmt"
	"reflect"
)

//Converter is a function type that converts a source object to a target type.
type Converter func (o interface{},target reflect.Type) (interface{},error)

//TimeConverter tries to convert an incoming interface to a 
// local/native time object. Basically an order of formats will be tried here.
func TimeConverter(o interface{},t reflect.Type) (ret interface{},err error) {
	if iv,ok := o.(int64); ok {
		//Assume unix timestamp (ms)
		return time.Unix(int64(iv/1000),0),nil
	} else if fv, ok := o.(float64); ok {
		return time.Unix(int64(fv/1000),0),nil
	} else if sv, ok := o.(string); ok {
		layouts := []string { time.RFC3339, time.RFC3339Nano, time.ANSIC, time.UnixDate, time.RubyDate, time.RFC822, time.RFC822Z, time.RFC850, time.RFC1123, time.RFC1123Z, time.Kitchen, time.Stamp, time.StampMilli, time.StampMicro, time.StampNano }

		for _,lay := range layouts {
			ret, err = time.Parse(lay,sv)
			if err == nil {
				return
			}
		}
		err = fmt.Errorf("No suitable time format identified: %s",sv)

	}
	err = fmt.Errorf("Cannot convert time object: %s",o)
	return
}

//String Converter tries to make a string out of the incoming object.
func StringConverter(o interface{}, t reflect.Type) (ret interface{}, err error) {
	switch reflect.TypeOf(o).Kind(){
		case reflect.String:
			ret = o
		case reflect.Float64,reflect.Float32:
			ret = fmt.Sprintf("%f",o)
		case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Uint,reflect.Uint32,reflect.Uint8,reflect.Uint16,reflect.Int64,reflect.Uint64:
			ret = fmt.Sprintf("%d",o)
		default:
			ret = fmt.Sprintf("%s",o)
	}
	return
}