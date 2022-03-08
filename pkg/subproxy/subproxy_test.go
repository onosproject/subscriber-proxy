// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package subproxy

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/onosproject/onos-lib-go/pkg/logging"
	"github.com/onosproject/sdcore-adapter/pkg/test/mocks"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var sp = subscriberProxy{
	AetherConfigAddress:   "onos-config.micro-onos.svc.cluster.local:5150",
	BaseWebConsoleURL:     "http://webui.omec.svc.cluster.local:5000",
	AetherConfigTarget:    "connectivity-service-v2",
	gnmiClient:            nil,
	PostTimeout:           0,
	retryInterval:         0,
	synchronizeDeviceFunc: nil,
}

func TestMain(m *testing.M) {
	log := logging.GetLogger("subscriber-proxy")
	log.SetLevel(logging.DebugLevel)
	clientHTTP = &mocks.MockHTTPClient{}
	os.Exit(m.Run())
}

func TestSubscriberProxy_addSubscriberByID(t *testing.T) {

	dgJSON, err := ioutil.ReadFile("./testdata/device.json")
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	gnmiMockClient := mocks.NewMockGnmiInterface(ctrl)
	sp.gnmiClient = gnmiMockClient

	gnmiMockClient.EXPECT().GetPath(gomock.Any(), "/enterprise", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, path string, target string, addr string) (*gpb.TypedValue, error) {
			return &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonVal{JsonVal: dgJSON},
			}, nil
		}).AnyTimes()

	var updSetRequests []*gpb.SetRequest
	gnmiMockClient.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, prefix *gpb.Path, target string, addr string, updates []*gpb.Update) error {
			updSetRequests = append(updSetRequests, &gpb.SetRequest{
				Update: updates,
			})
			return nil
		}).AnyTimes()

	respMock := ioutil.NopCloser(bytes.NewReader([]byte(`{}`)))
	httpMockClient := mocks.NewMockHTTPClient(ctrl)

	httpMockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 201,
			Body:       respMock,
		}, nil
	}).AnyTimes()

	clientHTTP = httpMockClient
	w := httptest.NewRecorder()
	router := gin.New()
	router.Use(getlogger(), gin.Recovery())
	router.POST("/api/subscriber/:ueId", sp.addSubscriberByID)
	payload := strings.NewReader(`{` + "" + `"plmnID": "26512",` + "" + `"ueId": "imsi-111222333444555",` + "" + `
	"OPc": "8e27b6af0e692e750f32667a3b14605d",` + "" + `"key": "8baf473f2f8fd09487cccbd7097c6862",` + "" + `
	"sequenceNumber": "16f3b3f70fc2",` + "" + `"DNN": "internet "` + "" + `}`)
	req, err := http.NewRequest("POST", "/api/subscriber/imsi-111222333444555", payload)
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Dest-Url", "http://webui.omec.svc.cluster.local:5000/api/subscriber")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	resp, err := ioutil.ReadAll(w.Body)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "{\"status\":\"success\"}", string(resp))
}

func TestSubscriberProxy_delSubscriberByID(t *testing.T) {

	ctrl := gomock.NewController(t)
	respMock := ioutil.NopCloser(bytes.NewReader([]byte(`{}`)))
	httpMockClient := mocks.NewMockHTTPClient(ctrl)

	httpMockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       respMock,
		}, nil
	}).AnyTimes()

	clientHTTP = httpMockClient
	w := httptest.NewRecorder()
	router := gin.New()
	router.Use(getlogger(), gin.Recovery())
	router.DELETE("/api/subscriber/:ueId", sp.delSubscriberByID)
	payload := strings.NewReader(`{` + "" + `"plmnID": "26512",` + "" + `"ueId": "imsi-111222333444555",` + "" + `
	"OPc": "8e27b6af0e692e750f32667a3b14605d",` + "" + `"key": "8baf473f2f8fd09487cccbd7097c6862",` + "" + `
	"sequenceNumber": "16f3b3f70fc2",` + "" + `"DNN": "internet "` + "" + `}`)
	req, err := http.NewRequest("DELETE", "/api/subscriber/imsi-111222333444555", payload)
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Dest-Url", "http://webui.omec.svc.cluster.local:5000/api/subscriber")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	resp, err := ioutil.ReadAll(w.Body)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "{\"status\":\"success\"}", string(resp))
}

func TestSubscriberProxy_getDevice(t *testing.T) {

	dgJSON, err := ioutil.ReadFile("./testdata/device.json")
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	gnmiMockClient := mocks.NewMockGnmiInterface(ctrl)
	sp.gnmiClient = gnmiMockClient

	gnmiMockClient.EXPECT().GetPath(gomock.Any(), "/enterprise", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, path string, target string, addr string) (*gpb.TypedValue, error) {
			return &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonVal{JsonVal: dgJSON},
			}, nil
		}).AnyTimes()

	device, err := sp.getDevice()
	assert.NoError(t, err)
	assert.NotNil(t, device.Enterprises)
	assert.Len(t, device.Enterprises.Enterprise, 3)
	assert.NotNil(t, device.Enterprises.Enterprise["defaultent"].Site["defaultent-defaultsite"])
	assert.Equal(t, "defaultent-defaultsite",
		*device.Enterprises.Enterprise["defaultent"].Site["defaultent-defaultsite"].SiteId)
	assert.Len(t, device.Enterprises.Enterprise["starbucks"].Site, 2)
	assert.Equal(t, "Seattle", *device.Enterprises.Enterprise["starbucks"].Site["starbucks-seattle"].DisplayName)
}

func TestSubscriberProxy_updateImsiDeviceGroup(t *testing.T) {

	dgJSON, err := ioutil.ReadFile("./testdata/device.json")
	assert.NoError(t, err)

	ctrl := gomock.NewController(t)
	gnmiMockClient := mocks.NewMockGnmiInterface(ctrl)
	sp.gnmiClient = gnmiMockClient

	gnmiMockClient.EXPECT().GetPath(gomock.Any(), "/enterprise", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, path string, target string, addr string) (*gpb.TypedValue, error) {
			return &gpb.TypedValue{
				Value: &gpb.TypedValue_JsonVal{JsonVal: dgJSON},
			}, nil
		}).AnyTimes()

	var updSetRequests []*gpb.SetRequest
	gnmiMockClient.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, prefix *gpb.Path, target string, addr string, updates []*gpb.Update) error {
			updSetRequests = append(updSetRequests, &gpb.SetRequest{
				Update: updates,
			})
			return nil
		}).AnyTimes()

	// IMSI will be added to default site
	imsiValue := uint64(111222333444555)
	err = sp.updateImsiDeviceGroup(imsiValue)
	assert.NoError(t, err)
	assert.NotNil(t, updSetRequests)
	assert.Len(t, updSetRequests, 1)

	//IMSI already exist in site
	updSetRequests = nil
	imsiValue = uint64(123456001000001)
	err = sp.updateImsiDeviceGroup(imsiValue)
	assert.NoError(t, err)
	assert.Len(t, updSetRequests, 0)

	// IMSI doesn't exist will be added to existing site
	updSetRequests = nil
	imsiValue = uint64(123456001000005)
	err = sp.updateImsiDeviceGroup(imsiValue)
	assert.NoError(t, err)
	assert.NotNil(t, updSetRequests)
	assert.Len(t, updSetRequests, 1)

}
