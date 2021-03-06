// Code generated by protoc-gen-gogo.
// source: stream_out.proto
// DO NOT EDIT!

package garden

import proto "github.com/gogo/protobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type StreamOutRequest struct {
	Handle           *string `protobuf:"bytes,1,req,name=handle" json:"handle,omitempty"`
	SrcPath          *string `protobuf:"bytes,2,req,name=src_path" json:"src_path,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *StreamOutRequest) Reset()         { *m = StreamOutRequest{} }
func (m *StreamOutRequest) String() string { return proto.CompactTextString(m) }
func (*StreamOutRequest) ProtoMessage()    {}

func (m *StreamOutRequest) GetHandle() string {
	if m != nil && m.Handle != nil {
		return *m.Handle
	}
	return ""
}

func (m *StreamOutRequest) GetSrcPath() string {
	if m != nil && m.SrcPath != nil {
		return *m.SrcPath
	}
	return ""
}

type StreamOutResponse struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *StreamOutResponse) Reset()         { *m = StreamOutResponse{} }
func (m *StreamOutResponse) String() string { return proto.CompactTextString(m) }
func (*StreamOutResponse) ProtoMessage()    {}

func init() {
}
