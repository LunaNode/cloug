package api

import (
	"os"
	"testing"
	"time"
)

const TEST_REGION = "toronto"
const TEST_OBJECT_LABEL = "test"
const TEST_PLAN_ID = 1
const TEST_IMAGE_ID = 26
const TEST_VOLUME_SIZE_GB = 50

func getTestAPI(t *testing.T) *API {
	apiID := os.Getenv("LUNANODE_TEST_ID")
	apiKey := os.Getenv("LUNANODE_TEST_KEY")
	if apiID == "" || apiKey == "" {
		t.Fatalf("LUNANODE_TEST_ID and LUNANODE_TEST_KEY environmental variables must be set")
	}
	api, err := MakeAPI(apiID, apiKey)
	if err != nil {
		t.Fatalf("error initializing API instance: %v", err)
	}
	return api
}

func waitForTestVM(t *testing.T, api *API, vmID string) {
	for {
		_, info, err := api.VmInfo(vmID)
		if err != nil {
			t.Fatalf("error getting info on created VM: %v", err)
		}
		if info.Status == "Online" {
			break
		} else if info.Status != "BUILD" {
			t.Fatalf("unexpected VM status %s", info.Status)
		}
		time.Sleep(5 * time.Second)
	}
}

func waitForTestVolume(t *testing.T, api *API, volumeID int) {
	for {
		volume, err := api.VolumeInfo(TEST_REGION, volumeID)
		if err != nil {
			t.Fatalf("error getting info on created volume: %v", err)
		}
		if volume.Status == "available" {
			break
		} else if volume.Status != "creating" && volume.Status != "downloading" {
			t.Fatalf("unexpected volume status %s", volume.Status)
		}
		time.Sleep(5 * time.Second)
	}
}

func cleanTestVM(t *testing.T, api *API, vmID string) {
	if err := api.VmDelete(vmID); err != nil {
		t.Fatalf("error deleting VM: %v", err)
	}
}

func cleanTestVolume(t *testing.T, api *API, volumeID int) {
	// give some time to detach, in case it was in-use and VM was just deleted
	volume, err := api.VolumeInfo(TEST_REGION, volumeID)
	if err != nil {
		t.Fatalf("error getting info on volume while deleting: %v", err)
	}
	if volume.Status == "in-use" {
		time.Sleep(15 * time.Second)
	}
	if err := api.VolumeDelete(TEST_REGION, volumeID); err != nil {
		t.Fatalf("error deleting volume: %v", err)
	}
}

func TestVmCreateVolume(t *testing.T) {
	api := getTestAPI(t)
	imageID := TEST_IMAGE_ID
	volumeID, err := api.VolumeCreate(TEST_REGION, TEST_VOLUME_SIZE_GB, &imageID)
	if err != nil {
		t.Fatalf("error creating volume: %v", err)
	}
	defer cleanTestVolume(t, api, volumeID)
	waitForTestVolume(t, api, volumeID)
	vmID, err := api.VmCreateVolume(TEST_REGION, TEST_OBJECT_LABEL, TEST_PLAN_ID, volumeID)
	if err != nil {
		t.Fatalf("error creating VM: %v", err)
	}
	defer cleanTestVM(t, api, vmID)
	waitForTestVM(t, api, vmID)
}

func TestVmCreateImage(t *testing.T) {
	api := getTestAPI(t)
	vmID, err := api.VmCreateImage(TEST_REGION, TEST_OBJECT_LABEL, TEST_PLAN_ID, TEST_IMAGE_ID)
	if err != nil {
		t.Fatalf("error creating VM: %v", err)
	}
	defer cleanTestVM(t, api, vmID)
	waitForTestVM(t, api, vmID)
}

func TestVmActions(t *testing.T) {
	api := getTestAPI(t)
	vmID, err := api.VmCreateImage(TEST_REGION, TEST_OBJECT_LABEL, TEST_PLAN_ID, TEST_IMAGE_ID)
	if err != nil {
		t.Fatalf("error creating VM: %v", err)
	}
	defer cleanTestVM(t, api, vmID)
	waitForTestVM(t, api, vmID)
	api.VmStop(vmID)
	_, info, err := api.VmInfo(vmID)
	if err != nil {
		t.Fatalf("error getting info on created VM: %v", err)
	}
	if info.Status != "Powering off" && info.Status != "Offline" {
		t.Fatalf("unexpected VM status %s after stopping", info.Status)
	}
}
