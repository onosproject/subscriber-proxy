// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package subproxy

import (
	"fmt"
	"github.com/gin-gonic/gin"
	models "github.com/onosproject/config-models/modelplugin/aether-2.0.0/aether_2_0_0"
	"github.com/onosproject/onos-lib-go/pkg/errors"
	"github.com/onosproject/sdcore-adapter/pkg/gnmiclient"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//init
func init() {
	clientHTTP = &http.Client{}
}

//HTTPClient interface
//go:generate mockgen -destination=../test/mocks/mock_http.go -package=mocks github.com/onosproject/sdcore-adapter/pkg/subproxy HTTPClient
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

//addSubscriberByID
func (s *subscriberProxy) addSubscriberByID(c *gin.Context) {
	log.Infof("Received One Subscriber Data")
	ueID := c.Param("ueId")
	var payload []byte
	if c.Request.Body != nil {
		payload, _ = ioutil.ReadAll(c.Request.Body)
	}

	if !strings.HasPrefix(ueID, "imsi-") {
		log.Warn("Ue Id format is invalid ")
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	log.Infof("Received subscriber id : %s ", ueID)

	split := strings.Split(ueID, "-")
	imsiValue, err := strconv.ParseUint(split[1], 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	//Getting gnmi context
	s.gnmiContext = NewGnmiContext(c)
	err = s.updateImsiDeviceGroup(imsiValue)
	if err != nil {
		jsonByte, okay := getJSONResponse(err.Error())
		if okay != nil {
			log.Warn(err.Error())
		}
		c.Data(http.StatusInternalServerError, "application/json", jsonByte)
		return
	}

	//Get destination url from the header
	destUrl := c.Request.Header.Get("Dest-Url")
	log.Infof("Destination URL : ", destUrl)

	if destUrl == "" {
		jsonByte, okay := getJSONResponse("No Target URL received from SimApp")
		if okay != nil {
			log.Warn(err.Error())
		}
		c.Data(http.StatusInternalServerError, "application/json", jsonByte)
		return
	}

	resp, err := ForwardReqToEndpoint(destUrl, payload, s.PostTimeout)
	if err != nil {
		jsonByte, okay := getJSONResponse(err.Error())
		if okay != nil {
			log.Warn(err.Error())
		}
		c.Data(http.StatusInternalServerError, "application/json", jsonByte)
		return
	}
	if resp.StatusCode != 201 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			jsonByte, okay := getJSONResponse(err.Error())
			if okay != nil {
				log.Warn(err.Error())
			}
			c.Data(http.StatusInternalServerError, "application/json", jsonByte)
			return
		}

		bodyBytes, err = getJSONResponse(string(bodyBytes))
		if err != nil {
			log.Warn(err.Error())
		}
		c.Data(resp.StatusCode, "application/json", bodyBytes)
		return
	}

	c.JSON(resp.StatusCode, gin.H{"status": "success"})
}

//InitGnmiContext
func (s *subscriberProxy) InitGnmiContext() error {

	var err error
	s.gnmiClient, s.token, err = gnmiclient.NewGnmiWithInterceptor(s.AetherConfigAddress, time.Second*15)
	if err != nil {
		log.Errorf("Error opening gNMI client %s", err.Error())
		s.gnmiClient = nil //ensure it's nil
		return err
	}
	return nil
}

//getDevice
func (s *subscriberProxy) getDevice() (*models.Device, error) {

	if s.gnmiClient == nil {
		err := s.InitGnmiContext()
		if err != nil {
			return nil, err
		}
	}

	//Append the auth token if oid issuer is configured
	openIDIssuer := os.Getenv("OIDC_SERVER_URL")
	if len(strings.TrimSpace(openIDIssuer)) > 0 {
		s.gnmiContext = metadata.AppendToOutgoingContext(s.gnmiContext, authorization, s.token)
	}

	//Getting Device Group only
	origValDg, err := s.gnmiClient.GetPath(s.gnmiContext, "/enterprise", s.AetherConfigTarget, s.AetherConfigAddress)
	if err != nil {
		log.Error("GetPath call failed with error ", err.Error())
		//Check if the token is expired and retry with new token
		if (len(strings.TrimSpace(openIDIssuer)) > 0) && (strings.Contains(err.Error(), "expired")) {
			log.Info("Retrying with fresh token ")
			err = s.InitGnmiContext()
			if err != nil {
				return nil, err
			}
			origValDg, err = s.gnmiClient.GetPath(s.gnmiContext, "/enterprise", s.AetherConfigTarget, s.AetherConfigAddress)
			if err != nil {
				return nil, errors.NewInvalid("failed to get the current state from onos-config: %v", err.Error())
			}
		} else {
			return nil, errors.NewInvalid("failed to get the current state from onos-config: %v", err.Error())
		}
	}

	device := &models.Device{}
	// Convert the JSON config into a Device structure for Device Group
	origJSONBytes := origValDg.GetJsonVal()
	if len(origJSONBytes) > 0 {
		if err := models.Unmarshal(origJSONBytes, device); err != nil {
			log.Error("Failed to unmarshal json", err)
			return nil, errors.NewInvalid("failed to unmarshal json", err)
		}
	}

	//TODO see if we can only the SIM Objects instead of the entire Enterprise tree
	return device, nil
}

//updateImsiDeviceGroup
func (s *subscriberProxy) updateImsiDeviceGroup(imsi uint64) error {

	// Getting the current configuration from the ROC for Site and Devices
	device, err := s.getDevice()
	if err != nil {
		return err
	}
	// Check if the SimCard instance already exist for the imsi
	if checkIfSimCardExist(device, imsi) {
		log.Infof("Sim with imsi %u already exists", imsi)
		return nil
	}
	//find the Site for the imsi if is no site found then it will added to default site under default enterprize
	ent, site, err := findSiteForTheImsi(device, imsi)
	if err != nil {
		return err
	}
	if site == nil {
		log.Infof("No sites found for this imsi %s", imsi)
		//Set the default site and ent
		ent = device.Enterprises.Enterprise["defaultent"]
		site = ent.Site["defaultent-defaultsite"]
	}
	return s.addSimObjectToSite(site, ent, imsi)

}

//addSimObjectToSite adds Imsi to default group expect the group already exists
func (s *subscriberProxy) addSimObjectToSite(site *models.OnfEnterprise_Enterprises_Enterprise_Site, ent *models.OnfEnterprise_Enterprises_Enterprise, imsi uint64) error {
	log.Infof("SiteID =  %s and entID = %s ", *site.SiteId, *ent.EnterpriseId)

	noOfSim := len(site.SimCard) + 1
	log.Infof("Sim Card count : %d ", noOfSim)

	prefix := gnmiclient.StringToPath(fmt.Sprintf("enterprises/enterprise[enterprise-id=%s]/site[site-id=%s]", *ent.EnterpriseId, *site.SiteId), s.AetherConfigTarget)

	// Build up a list of gNMI updates to apply
	updates := []*gpb.Update{}
	//TODO investigate append option - till that time get the previsous sim objects and append it
	for _, sim := range site.SimCard {
		updStr := fmt.Sprintf("sim-card[sim-id=%s]/display-name", *sim.SimId)
		updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateString(updStr, s.AetherConfigTarget, sim.DisplayName))
		updStr = fmt.Sprintf("sim-card[sim-id=%s]/imsi", *sim.SimId)
		updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateUInt64(updStr, s.AetherConfigTarget, sim.Imsi))
		updStr = fmt.Sprintf("sim-card[sim-id=%s]/description", *sim.SimId)
		updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateString(updStr, s.AetherConfigTarget, sim.Description))
		updStr = fmt.Sprintf("sim-card[sim-id=%s]/iccid", *sim.SimId)
		updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateUInt64(updStr, s.AetherConfigTarget, sim.Iccid))
	}

	iccid := uint64(1234)                   //TODO see how we can get this value
	simID := fmt.Sprintf("sim-%d", noOfSim) //TODO need to have concrete logic for unique name
	simDisplayName := fmt.Sprintf("Sim-%d", noOfSim)
	simDescription := fmt.Sprintf("Sim-%d description", noOfSim)

	updStr := fmt.Sprintf("sim-card[sim-id=%s]/display-name", simID)
	updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateString(updStr, s.AetherConfigTarget, &simDisplayName))
	updStr = fmt.Sprintf("sim-card[sim-id=%s]/imsi", simID)
	updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateUInt64(updStr, s.AetherConfigTarget, &imsi))
	updStr = fmt.Sprintf("sim-card[sim-id=%s]/description", simID)
	updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateString(updStr, s.AetherConfigTarget, &simDescription))
	updStr = fmt.Sprintf("sim-card[sim-id=%s]/iccid", simID)
	updates = gnmiclient.AddUpdate(updates, gnmiclient.UpdateUInt64(updStr, s.AetherConfigTarget, &iccid))

	// Apply them
	err := s.gnmiClient.Update(s.gnmiContext, prefix, s.AetherConfigTarget, s.AetherConfigAddress, updates)
	if err != nil {
		log.Errorf("Error while applying changes via gNMI %v", err)
		return errors.NewInternal("Error executing gNMI: %v", err)
	}
	return nil

}

//StartSubscriberProxy start the subscriber
func (s *subscriberProxy) StartSubscriberProxy(bindPort string, path string) error {
	router := gin.New()
	router.Use(getlogger(), gin.Recovery())
	router.POST(path, getlogger(), s.addSubscriberByID)
	err := router.Run("0.0.0.0" + bindPort)
	if err != nil {
		return err
	}
	return nil
}

//NewSubscriberProxy as Init method
func NewSubscriberProxy(aetherConfigTarget string, baseWebConsoleURL string, aetherConfigAddr string,
	postTimeout time.Duration) *subscriberProxy {
	sproxy := &subscriberProxy{
		AetherConfigAddress: aetherConfigAddr,
		AetherConfigTarget:  aetherConfigTarget,
		BaseWebConsoleURL:   baseWebConsoleURL,
		PostTimeout:         postTimeout,
	}
	return sproxy
}
