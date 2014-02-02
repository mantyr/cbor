package cbor

import (
	"fmt"
	"encoding/binary"
	"bytes"
	"unicode/utf8"
	"reflect"
)

const (
	majorOffset = 5
	additionalWipe = 7 << majorOffset
	majorWipe = 31
	additionalMax = 23
	additionalTypeIntFalse byte = 20
	additionalTypeIntTrue byte = 21
	additionalTypeIntNull byte = 22
	additionalTypeIntUndefined byte = 23
	additionalTypeIntUint8 byte = 24
	additionalTypeIntUint16 byte = 25
	additionalTypeIntUint32 byte = 26
	additionalTypeIntUint64 byte = 27
	additionalTypeFloat16 byte = 25
	additionalTypeFloat32 byte = 26
	additionalTypeFloat64 byte = 27
	additionalTypeBreak byte = 31
)

const (
	majorTypeUnsignedInt byte = iota << majorOffset
	majorTypeInt
	majorTypeByteString
	majorTypeUtf8String
	majorTypeArray
	majorTypeMap
	majorTypeTags
	majorTypeSimpleAndFloat
)

var additionalLength = map[byte]byte{
	additionalTypeIntUint8 : 1,
	additionalTypeIntUint16 : 2,
	additionalTypeIntUint32 : 4,
	additionalTypeIntUint64 : 8,
}

//exported decode func
func Decode(byteBuff *[]byte) (interface {}, error) {
	data, _, err := decode(*byteBuff)
	return data, err
}

//decode with offset
func decode(buff []byte) (interface {}, int64, error) {
	if len(buff) == 0 {
		return nil, -1, fmt.Errorf("Empty input byte array")
	}

	majorType := buff[0] & additionalWipe
	headerAdditionInfo := buff[0] & majorWipe
	var dataOffset int64 = 1
	if offset, ok := additionalLength[headerAdditionInfo]; ok {
		dataOffset += int64(offset)
	}

	switch majorType {
	case majorTypeUnsignedInt, majorTypeInt:
		number, err := decodeInt(headerAdditionInfo,buff[1:dataOffset])

		if err != nil {
			return nil, -1, err
		}

		if majorType == majorTypeInt {
			return -(number + 1), dataOffset, nil
		}

		return number, dataOffset, nil
	case majorTypeByteString, majorTypeUtf8String:
		length, err := decodeInt(headerAdditionInfo, buff[1:dataOffset])

		if err != nil {
			return nil, -1, err
		}
		newOffset := int64(dataOffset) + length
		return string(buff[dataOffset: newOffset]), newOffset, nil
	case majorTypeArray:
		array_cap, err := decodeInt(headerAdditionInfo, buff[1:dataOffset])

		if err != nil {
			return nil, -1, err
		}

		var out []interface {}

		offset := dataOffset
		for int(array_cap) > len(out) {
			data, newOffset, err1 := decode(buff[offset:])
			if err1 != nil {
				return nil, -1, err
			}

			out = append(out, data)
			offset += newOffset
		}

		return out, offset, nil
	case majorTypeMap:
		array_cap, err := decodeInt(headerAdditionInfo, buff[1:dataOffset])

		if err != nil {
			return nil, -1, err
		}

		out := map[interface{}]interface {}{}

		offset := dataOffset
		for int(array_cap) > len(out) {
			key, newOffset, key_err := decode(buff[offset:])
			if key_err != nil {
				return nil, -1, key_err
			}

			offset += newOffset

			value, newOffset, value_err := decode(buff[offset:])
			if value_err != nil {
				return nil, -1, value_err
			}

			out[key] = value

			offset += newOffset
		}

		return out, offset, nil
	case majorTypeTags:
		return nil, -1, fmt.Errorf("Tags not support")
	case majorTypeSimpleAndFloat:
		switch headerAdditionInfo {
		case additionalTypeIntFalse:
			return false, dataOffset, nil
		case additionalTypeIntTrue:
			return true, dataOffset, nil
		case additionalTypeIntNull:
			return nil, dataOffset, nil
		case additionalTypeFloat16:
			return nil, -1, fmt.Errorf("Float16 decode not support")
		case additionalTypeFloat32:
			var out float32
			err := unpack(buff[1:dataOffset], &out)
			return out, dataOffset, err
		case additionalTypeFloat64:
			var out float64
			err := unpack(buff[1:dataOffset], &out)
			return out, dataOffset, err
		}
	}

	return nil, -1, nil
}


//decode int
func decodeInt(headerAdditionInfo byte, buff []byte) (int64, error) {
	if headerAdditionInfo <= additionalMax {
		return int64(headerAdditionInfo), nil
	}

	var number int64
	var err error

	switch headerAdditionInfo {
	case additionalTypeIntUint8:
		return int64(buff[0]), nil
	case additionalTypeIntUint16:
		var out uint16
		err = unpack(buff, &out)
		number = int64(out)
	case additionalTypeIntUint32:
		var out uint32
		err = unpack(buff, &out)
		number = int64(out)
	default:
		var out uint64
		err = unpack(buff, &out)
		number = int64(out)
	}

	if err != nil {
		return 0, err
	}

	return number, nil
}

func unpack(byteBuff []byte, test interface{}) (error){
	buf := bytes.NewReader(byteBuff)
	err := binary.Read(buf, binary.BigEndian, test)
	return err
}

func Encode(variable interface{}) ([]byte, error) {
	if variable == nil {
		return encodeNil()
	}

	switch reflect.TypeOf(variable).Kind() {
	case reflect.Int:
		return encodeNumber(variable.(int))
	case reflect.String:
		return encodeString(variable.(string))
	case reflect.Array, reflect.Slice:
		return encodeArray(variable)
	case reflect.Map:
		return encodeMap(variable)
	case reflect.Bool:
		return encodeBool(variable.(bool))
	case reflect.Float32:
		return encodeFloat(variable.(float32), additionalTypeFloat32)
	case reflect.Float64:
		return encodeFloat(variable.(float64), additionalTypeFloat64)
	}

	return nil, nil
}

/**
	Encoding float32/float64
 */
func encodeFloat(number interface{}, additionalFloatType byte)([]byte, error){
	majorType := majorTypeSimpleAndFloat

	initByte, err := packInitByte(majorType, additionalFloatType)

	if err != nil {
		return []byte{}, err
	}

	var packedInfo []byte
	var errPack error

	switch additionalFloatType {
	case additionalTypeFloat32:
		packedInfo, errPack = pack(number.(float32))
	case additionalTypeFloat64:
		packedInfo, errPack = pack(number.(float64))
	default:
		packedInfo, errPack = nil, nil
	}

	if errPack != nil {
		return nil, errPack
	}

	return append(initByte, packedInfo...), nil
}

/**
	encoding nil
 */
func encodeNil() ([]byte, error){
	return packInitByte(majorTypeSimpleAndFloat, additionalTypeIntNull)
}

/**
	encode
 */
func encodeBool(variable bool) ([]byte, error){
	if variable {
		return packInitByte(majorTypeSimpleAndFloat, additionalTypeIntTrue)
	}

	return packInitByte(majorTypeSimpleAndFloat, additionalTypeIntFalse)
}

/**
	Encode array to CBOR binary string
 */
func encodeArray(variable interface{}) ([]byte, error) {
	majorType := majorTypeArray
	inputSlice := reflect.ValueOf(variable)
	length := inputSlice.Len()

	buff, err := packNumber(majorType, uint64(length))

	if err != nil {
		return nil, err
	}

	//array slice encode
	for i:=0; i < inputSlice.Len(); i++ {
		elementBuff, err := Encode(inputSlice.Index(i).Interface())

		if err != nil {
			return nil, err
		}

		buff = append(buff, elementBuff...)
	}

	return buff, nil
}

/**
	Encode map to CBOR binary string
 */
func encodeMap(variable interface{}) ([]byte, error) {
	majorType := majorTypeMap
	inputSlice := reflect.ValueOf(variable)
	length := inputSlice.Len()

	buff, err := packNumber(majorType, uint64(length))

	if err != nil {
		return nil, err
	}

	//map encode
	for _, key := range inputSlice.MapKeys() {
		keyBuff, keyErr := Encode(key.Interface())

		if keyErr != nil {
			return nil, keyErr
		}

		buff = append(buff, keyBuff...)

		elementBuff, elemErr := Encode(inputSlice.MapIndex(key).Interface())

		if elemErr != nil {
			return nil, elemErr
		}

		buff = append(buff, elementBuff...)
	}

	return buff, nil
}

/**
	Encode string to CBOR binary string
 */
func encodeString(variable string) ([]byte, error) {
	byteBuf := []byte(variable)

	majorType := majorTypeUtf8String

	if !utf8.Valid(byteBuf) {
		majorType = majorTypeByteString
	}

	initByte, err := packNumber(majorType, uint64(len(byteBuf)))

	if err != nil {
		return []byte{}, err
	}

	return append(initByte, byteBuf...), nil
}

/**
	Encode integer to CBOR binary string
 */
func encodeNumber(variable int) ([]byte, error) {
	var majorType = majorTypeUnsignedInt

	var unsignedVariable uint64

	if variable < 0 {
		majorType = majorTypeInt
		unsignedVariable = uint64(-(variable + 1))
	} else {
		unsignedVariable = uint64(variable)
	}

	byteArr, err := packNumber(majorType, unsignedVariable)
	return byteArr, err
}

/**
	Pack number helper
 */
func packNumber(majorType byte, number uint64) ([]byte, error){
	if number < additionalMax {
		return packInitByte(majorType, byte(number))
	}

	additionInfo := intTypeToCborType(number)

	initByte, err := packInitByte(majorType, additionInfo)

	if err != nil {
		return []byte{}, err
	}

	var packedInfo []byte
	var errPack error

	switch additionInfo	{
	case additionalTypeIntUint8:
		packedInfo, errPack = pack(uint8(number))
	case additionalTypeIntUint16:
		packedInfo, errPack = pack(uint16(number))
	case additionalTypeIntUint32:
		packedInfo, errPack = pack(uint32(number))
	default:
		packedInfo, errPack = pack(uint64(number))
	}

	if errPack != nil {
		return nil, errPack
	}

	return append(initByte, packedInfo...), nil
}

/**
	Helper for packing Go objects. Like in C, PHP function pack()
 */
func pack(packVariable interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.BigEndian, packVariable)

	if err != nil {
		return nil, fmt.Errorf("Cant pack init byte. %s", err)
	}

	return buf.Bytes(), nil
}

/**
	Pack initial bye
*/
func packInitByte(majorType byte, additionalInfo byte) ([]byte, error) {
	return pack(majorType | additionalInfo)
}

/**
	Get CBOR additional info type for number
*/
func intTypeToCborType(number uint64) (byte) {
	switch {
	case number < 256:
		return additionalTypeIntUint8
	case number < 65536:
		return additionalTypeIntUint16
	case number < 4294967296:
		return additionalTypeIntUint32
	default:
		return additionalTypeIntUint64
	}
}
