package gotojs

import (
	"fmt"
	"reflect"
)

//Converter is a function type that converts a source object to a target type.
type Converter func (o interface{},target reflect.Type) (interface{},error)

//TimeConverter integrates the ConvertTime function as a converter.
func TimeConverter(o interface{},t reflect.Type) (ret interface{},err error) {
	return ConvertTime(o)
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
