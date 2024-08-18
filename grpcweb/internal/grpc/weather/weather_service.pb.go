// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.3
// source: weather_service.proto

package weather

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LocationRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Latitude  float64 `protobuf:"fixed64,1,opt,name=latitude,proto3" json:"latitude,omitempty"`
	Longitude float64 `protobuf:"fixed64,2,opt,name=longitude,proto3" json:"longitude,omitempty"`
}

func (x *LocationRequest) Reset() {
	*x = LocationRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_weather_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LocationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LocationRequest) ProtoMessage() {}

func (x *LocationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_weather_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LocationRequest.ProtoReflect.Descriptor instead.
func (*LocationRequest) Descriptor() ([]byte, []int) {
	return file_weather_service_proto_rawDescGZIP(), []int{0}
}

func (x *LocationRequest) GetLatitude() float64 {
	if x != nil {
		return x.Latitude
	}
	return 0
}

func (x *LocationRequest) GetLongitude() float64 {
	if x != nil {
		return x.Longitude
	}
	return 0
}

type WeatherResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Temperature float64 `protobuf:"fixed64,1,opt,name=temperature,proto3" json:"temperature,omitempty"`
	Humidity    float64 `protobuf:"fixed64,2,opt,name=humidity,proto3" json:"humidity,omitempty"`
	Status      string  `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *WeatherResponse) Reset() {
	*x = WeatherResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_weather_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WeatherResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WeatherResponse) ProtoMessage() {}

func (x *WeatherResponse) ProtoReflect() protoreflect.Message {
	mi := &file_weather_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WeatherResponse.ProtoReflect.Descriptor instead.
func (*WeatherResponse) Descriptor() ([]byte, []int) {
	return file_weather_service_proto_rawDescGZIP(), []int{1}
}

func (x *WeatherResponse) GetTemperature() float64 {
	if x != nil {
		return x.Temperature
	}
	return 0
}

func (x *WeatherResponse) GetHumidity() float64 {
	if x != nil {
		return x.Humidity
	}
	return 0
}

func (x *WeatherResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

var File_weather_service_proto protoreflect.FileDescriptor

var file_weather_service_proto_rawDesc = []byte{
	0x0a, 0x15, 0x77, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x77, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72,
	0x22, 0x4b, 0x0a, 0x0f, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x61, 0x74, 0x69, 0x74, 0x75, 0x64, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x01, 0x52, 0x08, 0x6c, 0x61, 0x74, 0x69, 0x74, 0x75, 0x64, 0x65, 0x12,
	0x1c, 0x0a, 0x09, 0x6c, 0x6f, 0x6e, 0x67, 0x69, 0x74, 0x75, 0x64, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x09, 0x6c, 0x6f, 0x6e, 0x67, 0x69, 0x74, 0x75, 0x64, 0x65, 0x22, 0x67, 0x0a,
	0x0f, 0x57, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x20, 0x0a, 0x0b, 0x74, 0x65, 0x6d, 0x70, 0x65, 0x72, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0b, 0x74, 0x65, 0x6d, 0x70, 0x65, 0x72, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x75, 0x6d, 0x69, 0x64, 0x69, 0x74, 0x79, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x01, 0x52, 0x08, 0x68, 0x75, 0x6d, 0x69, 0x64, 0x69, 0x74, 0x79, 0x12, 0x16,
	0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x32, 0x99, 0x01, 0x0a, 0x0e, 0x57, 0x65, 0x61, 0x74, 0x68,
	0x65, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x40, 0x0a, 0x0a, 0x47, 0x65, 0x74,
	0x57, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x12, 0x18, 0x2e, 0x77, 0x65, 0x61, 0x74, 0x68, 0x65,
	0x72, 0x2e, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x18, 0x2e, 0x77, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x2e, 0x57, 0x65, 0x61, 0x74,
	0x68, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x45, 0x0a, 0x0d, 0x53,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x57, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x12, 0x18, 0x2e, 0x77,
	0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x2e, 0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x77, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72,
	0x2e, 0x57, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x30, 0x01, 0x42, 0x0b, 0x5a, 0x09, 0x2e, 0x2f, 0x77, 0x65, 0x61, 0x74, 0x68, 0x65, 0x72, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_weather_service_proto_rawDescOnce sync.Once
	file_weather_service_proto_rawDescData = file_weather_service_proto_rawDesc
)

func file_weather_service_proto_rawDescGZIP() []byte {
	file_weather_service_proto_rawDescOnce.Do(func() {
		file_weather_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_weather_service_proto_rawDescData)
	})
	return file_weather_service_proto_rawDescData
}

var file_weather_service_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_weather_service_proto_goTypes = []any{
	(*LocationRequest)(nil), // 0: weather.LocationRequest
	(*WeatherResponse)(nil), // 1: weather.WeatherResponse
}
var file_weather_service_proto_depIdxs = []int32{
	0, // 0: weather.WeatherService.GetWeather:input_type -> weather.LocationRequest
	0, // 1: weather.WeatherService.StreamWeather:input_type -> weather.LocationRequest
	1, // 2: weather.WeatherService.GetWeather:output_type -> weather.WeatherResponse
	1, // 3: weather.WeatherService.StreamWeather:output_type -> weather.WeatherResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_weather_service_proto_init() }
func file_weather_service_proto_init() {
	if File_weather_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_weather_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*LocationRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_weather_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*WeatherResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_weather_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_weather_service_proto_goTypes,
		DependencyIndexes: file_weather_service_proto_depIdxs,
		MessageInfos:      file_weather_service_proto_msgTypes,
	}.Build()
	File_weather_service_proto = out.File
	file_weather_service_proto_rawDesc = nil
	file_weather_service_proto_goTypes = nil
	file_weather_service_proto_depIdxs = nil
}
