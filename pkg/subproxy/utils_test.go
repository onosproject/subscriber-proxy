// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
package subproxy

import (
	"bytes"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/onosproject/sdcore-adapter/pkg/test/mocks"
	"github.com/stretchr/testify/assert"

	//"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	//"strings"
	"testing"
	"time"
)

func TestForwardReqToEndpoint(t *testing.T) {

	type args struct {
		postURI     string
		payload     []byte
		postTimeout time.Duration
		method      string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"4gurl",
			args{
				postURI:     "http://config4g:5000/api/subscriber/imsi-208014567891201",
				payload:     []byte("{\"plmnID\":\"20893\",\"ueId\":\"imsi-208014567891201\",\"OPc\":\"8e27b6af0e692e750f32667a3b14605d\",\"key\":\"8baf473f2f8fd09487cccbd7097c6862\",\"sequenceNumber\":\"16f3b3f70fc2\",\"DNN\": \"internet\"}"),
				postTimeout: 1 * time.Second,
				method:      http.MethodPost,
			}, 201,
		},
		{"5gurl",
			args{
				postURI:     "http://webui:5000/api/subscriber/imsi-208014567891201",
				payload:     []byte("{\"plmnID\":\"20893\",\"ueId\":\"imsi-208014567891201\",\"OPc\":\"8e27b6af0e692e750f32667a3b14605d\",\"key\":\"8baf473f2f8fd09487cccbd7097c6862\",\"sequenceNumber\":\"16f3b3f70fc2\",\"DNN\": \"internet\"}"),
				postTimeout: 1 * time.Second,
				method:      http.MethodPost,
			}, 201,
		},
		{"delurl",
			args{
				postURI:     "http://webui:5000/api/subscriber/imsi-208014567891201",
				payload:     []byte("{\"plmnID\":\"20893\",\"ueId\":\"imsi-208014567891201\",\"OPc\":\"8e27b6af0e692e750f32667a3b14605d\",\"key\":\"8baf473f2f8fd09487cccbd7097c6862\",\"sequenceNumber\":\"16f3b3f70fc2\",\"DNN\": \"internet\"}"),
				postTimeout: 1 * time.Second,
				method:      http.MethodDelete,
			}, 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			respMock := ioutil.NopCloser(bytes.NewReader([]byte(`{"status":"success"}`)))
			httpMockClient := mocks.NewMockHTTPClient(ctrl)
			clientHTTP = httpMockClient

			httpMockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {

				log.Infof(" from Http mock client ...%v", req.URL)
				assert.Equal(t, tt.args.postURI, fmt.Sprintf("%v", req.URL))
				statusCode := 201
				if tt.args.method == http.MethodDelete {
					statusCode = 200
				}
				return &http.Response{
					StatusCode: statusCode,
					Body:       respMock,
					Header:     make(http.Header),
				}, nil
			}).AnyTimes()

			got, err := ForwardReqToEndpoint(tt.args.postURI, tt.args.payload, tt.args.postTimeout, tt.args.method)
			if err != nil {
				t.Errorf("ForwardReqToEndpoint() error = %v", err)
				return
			}
			resp, err := ioutil.ReadAll(got.Body)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.want, got.StatusCode)
		})
	}
}
