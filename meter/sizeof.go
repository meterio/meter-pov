package meter

import (
	"reflect"
)

var Size []uint64

// Sizeof Function Will Find Approximate Size of Object in Bytes
func Sizeof(i interface{}) (size uint64) {
	size = 0
	sizeof(reflect.ValueOf(i), &size)
	return
}

// sizeof  private function which used to calculate size of object
func sizeof(val reflect.Value, sizeptr *uint64) {
	if val.Kind() >= reflect.Bool && val.Kind() <= reflect.Complex128 {
		(*sizeptr) += Size[val.Kind()]
		return
	}
	switch val.Kind() {
	case reflect.String:
		(*sizeptr) += Size[reflect.String] * uint64(val.Len())
	case reflect.Array:
		/*
		   Then iterate through the array and get recursively call the size method.
		   If all element hold the same value calculate size for one and multiply it with
		   length of array. Wont wonk correctly if the array holds slices or any dynamic
		   elements
		*/
		for i := 0; i < val.Len(); i++ {
			sizeof(val.Index(i), (sizeptr))
		}

	case reflect.Interface:
		/*
		   First we need to get the underlying object of Interface in golang using the Elem()
		   And then we need to recursively call this function
		*/
		temp := val.Elem()
		sizeof(temp, (sizeptr))

	case reflect.Map:
		for _, key := range val.MapKeys() {
			/*
			   get the size of key by calling the size function
			*/
			sizeof(key, (sizeptr))

			/*
			   get the value pointed by the key and then recursively compute its size
			*/
			mapVal := val.MapIndex(key)
			sizeof(mapVal, sizeptr)
		}

	case reflect.Ptr:
		prtVal := val.Elem()
		/*
		   If the pointer is invalid or the pointer is nil then return without updating the size
		*/
		if !prtVal.IsValid() {
			return
		}

		sizeof(val.Elem(), sizeptr)

	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			sizeof(val.Index(i), sizeptr)
		}

	case reflect.Struct:
		/*
		   Didn't consider the NumMethod here. Don't this that this is necessary or is it??
		   Need to verify this...
		*/
		for i := 0; i < val.NumField(); i++ {
			sizeof(val.Field(i), sizeptr)
		}

	case reflect.UnsafePointer:
		/*
		   This allows conversion between elements. Dont think this should this is used in calculating
		   size
		*/
	case reflect.Func:
		// How to handle function pointers
	case reflect.Chan:
		// Don't think this case has to be handled as it will be produced and consumed
	default:
		return
	}
	return
}
