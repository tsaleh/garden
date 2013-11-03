package messagereader_test

import (
	"bytes"

	"code.google.com/p/gogoprotobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/messagereader"
	protocol "github.com/vito/garden/protocol"
)

var _ = Describe("Reading request messages over the wire", func() {
	Context("when a request is received", func() {
		It("returns the request and no error", func() {
			payload := protocol.Messages(&protocol.EchoRequest{
				Message: proto.String("some-message"),
			})

			request, err := messagereader.ReadRequest(payload)
			Expect(err).ToNot(HaveOccured())
			Expect(request).To(Equal(
				&protocol.EchoRequest{
					Message: proto.String("some-message"),
				},
			))
		})
	})

	Context("when the connection is broken", func() {
		It("returns an error", func() {
			payload := protocol.Messages(&protocol.PingRequest{})

			bogusPayload := bytes.NewBuffer(payload.Bytes()[0 : payload.Len()-1])

			_, err := messagereader.ReadRequest(bogusPayload)
			Expect(err).To(HaveOccured())
		})
	})
})