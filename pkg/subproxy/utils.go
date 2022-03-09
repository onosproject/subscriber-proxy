// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package subproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	models "github.com/onosproject/config-models/modelplugin/aether-2.0.0/aether_2_0_0"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	sync "github.com/onosproject/sdcore-adapter/pkg/synchronizer"
	"google.golang.org/grpc/metadata"
	"net/http"
	"time"
)

const (
	authorization = "Authorization"
	host          = "Host"
	userAgent     = "User-Agent"
	remoteAddr    = "remoteAddr"
)

//get logger
func getlogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		// Process request
		c.Next()
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if raw != "" {
			path = path + "?" + raw
		}
		log.Debugf("| %3d | %15s | %-7s | %s | %s",
			statusCode, clientIP, method, path, errorMessage)
	}
}

// get JSON from
func getJSONResponse(msg string) ([]byte, error) {
	responseData := make(map[string]interface{})
	responseData["status"] = msg
	jsonData, err := json.Marshal(responseData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

//Check site for this imsi
func findSiteForTheImsi(device *models.Device, imsi uint64) (*models.OnfEnterprise_Enterprises_Enterprise, *models.OnfEnterprise_Enterprises_Enterprise_Site, error) {

	for _, ent := range device.Enterprises.Enterprise {
		for _, site := range ent.Site {

			// Skip for default site
			if *site.SiteId == "defaultent-defaultsite" {
				continue
			}

			maskedImsi, err := sync.MaskSubscriberImsiDef(site.ImsiDefinition, imsi) // mask off the MCC/MNC/EntId
			log.Info("[Debug] maskedImsi ", maskedImsi)
			if err != nil {
				return nil, nil, errors.NewInvalid("Failed to mask the subscriber: %v", err)
			}
			entID := uint32(0)
			if site.ImsiDefinition.Enterprise != nil {
				entID = *site.ImsiDefinition.Enterprise
			}
			siteImsiValue, err := sync.FormatImsi(*site.ImsiDefinition.Format, *site.ImsiDefinition.Mcc,
				*site.ImsiDefinition.Mnc, entID, maskedImsi)
			if err != nil {
				return nil, nil, errors.NewInvalid("Failed to mask the subscriber: %v", err)
			}
			log.Info("[Debug] siteImsiValue = ", siteImsiValue)

			if imsi == siteImsiValue {
				log.Debugf("Found the site for imsi : ", *site.SiteId)
				return ent, site, nil
			}

		}
	}

	return nil, nil, nil
}

//Check if any DeviceGroups contains this imsi
func checkIfSimCardExist(device *models.Device, imsi uint64) bool {

	for _, ent := range device.Enterprises.Enterprise {
		for _, site := range ent.Site {
			for _, simCard := range site.SimCard {
				log.Info("[Debug] Sim Imsi ", *simCard.Imsi)
				if imsi == *simCard.Imsi {
					return true
				}
			}
		}
	}
	return false
}

// ForwardReqToEndpoint will Call webui API for subscriber provision on the SD-Core
func ForwardReqToEndpoint(destURL string, payload []byte, postTimeout time.Duration, method string) (*http.Response, error) {
	log.Info("Forwarding Dest URI :  ", destURL)

	var req *http.Request
	var err error
	if method == http.MethodPost {
		req, err = http.NewRequest(method, destURL, bytes.NewBuffer(payload))
		if err != nil {
			return nil, errors.NewInvalid("Error while connecting  ", err.Error())
		}
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/json")
	} else if method == http.MethodDelete {
		req, err = http.NewRequest(method, destURL, nil)
		if err != nil {
			return nil, errors.NewInvalid("Error while connecting  ", err.Error())
		}
	} else {
		return nil, errors.NewInvalid("Method not support ", method)
	}

	resp, err := clientHTTP.Do(req)
	if err != nil {
		log.Error("Error forwarding request ", err.Error())
		return resp, errors.NewInvalid(err.Error())
	}
	defer resp.Body.Close()

	log.Info("Received response ", resp)

	return resp, nil
}

// NewGnmiContext - convert the gin context in to a gRPC Context
func NewGnmiContext(httpContext *gin.Context) context.Context {

	if len(httpContext.Request.Header.Get(authorization)) > 0 {
		return metadata.AppendToOutgoingContext(context.Background(),
			authorization, httpContext.Request.Header.Get(authorization),
			host, httpContext.Request.Host,
			"ua", httpContext.Request.Header.Get(userAgent), // `User-Agent` would be over written by gRPC
			remoteAddr, httpContext.Request.RemoteAddr)
	}
	return metadata.AppendToOutgoingContext(context.Background(),
		host, httpContext.Request.Host,
		"ua", httpContext.Request.Header.Get(userAgent), // `User-Agent` would be over written by gRPC
		remoteAddr, httpContext.Request.RemoteAddr)
}
