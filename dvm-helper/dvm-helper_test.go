package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fatih/color"
	"github.com/ryanuber/go-glob"
	"github.com/stretchr/testify/assert"
)

type requestHandler func(w http.ResponseWriter, r *http.Request)

func docker1_10_3_Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case glob.Glob("*/version", r.RequestURI):
		fmt.Fprintln(w, `{
     "Version": "swarm/1.2.3",
     "ApiVersion": "1.22"
}`)
	default:
		w.WriteHeader(404)
	}
}

func docker1_12_1_Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch {
	case glob.Glob("*/version", r.RequestURI):
		fmt.Fprintln(w, `{
     "Version": "1.12.1"
}`)
	default:
		w.WriteHeader(404)
	}
}

func githubReleasesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.RequestURI {
	case "/repos/docker/docker/releases?per_page=100":
		fmt.Fprintln(w, loadTestData("github-docker-releases.json"))
	default:
		w.WriteHeader(404)
	}
}

func createMockDVM(dockerHandler requestHandler) (docker *httptest.Server, github *httptest.Server) {
	github = httptest.NewServer(http.HandlerFunc(githubReleasesHandler))
	githubUrlOverride = github.URL + "/"

	if dockerHandler != nil {
		docker = httptest.NewServer(http.HandlerFunc(dockerHandler))
	}
	return
}

func TestDetectOldVersion(t *testing.T) {
	docker, github := createMockDVM(docker1_10_3_Handler)
	defer docker.Close()
	defer github.Close()

	os.Setenv("DOCKER_HOST", docker.URL)
	os.Setenv("DOCKER_TLS_VERIFY", "0")
	os.Unsetenv("DOCKER_CERT_PATH")

	outputCapture := &bytes.Buffer{}
	color.Output = outputCapture

	dvm := makeCliApp()
	dvm.Run([]string{"dvm", "--debug", "detect"})

	version := os.Getenv("DOCKER_VERSION")
	assert.Equal(t, version, "1.10.3", "Detected the wrong version")

	output := outputCapture.String()
	assert.NotEmpty(t, output, "Should have captured stdout")
	assert.Contains(t, output, "Detected client version: 1.10.3", "Should have printed the detected version")
}

func TestDetectVersion(t *testing.T) {
	docker, github := createMockDVM(docker1_12_1_Handler)
	defer docker.Close()
	defer github.Close()

	outputCapture := &bytes.Buffer{}
	color.Output = outputCapture

	os.Setenv("DOCKER_HOST", docker.URL)
	os.Setenv("DOCKER_TLS_VERIFY", "0")
	os.Unsetenv("DOCKER_CERT_PATH")

	dvm := makeCliApp()
	dvm.Run([]string{"dvm", "--debug", "detect"})

	version := os.Getenv("DOCKER_VERSION")
	assert.Equal(t, version, "1.12.1", "Detected the wrong version")

	output := outputCapture.String()
	assert.NotEmpty(t, output, "Should have captured stdout")
	assert.Contains(t, output, "Detected client version: 1.12.1", "Should have printed the detected version")
}

func TestListRemote(t *testing.T) {
	_, github := createMockDVM(nil)
	defer github.Close()

	outputCapture := &bytes.Buffer{}
	color.Output = outputCapture

	dvm := makeCliApp()
	dvm.Run([]string{"dvm", "--debug", "list-remote", "1.12"})

	output := outputCapture.String()
	assert.NotEmpty(t, output, "Should have captured stdout")
	assert.NotContains(t, output, "1.12.5-rc1", "Should not have listed a prerelease version")

}

func TestListRemoteWithPrereleases(t *testing.T) {
	_, github := createMockDVM(nil)
	defer github.Close()

	outputCapture := &bytes.Buffer{}
	color.Output = outputCapture

	dvm := makeCliApp()
	dvm.Run([]string{"dvm-helper", "--debug", "list-remote", "--pre", "1.12"})

	output := outputCapture.String()
	assert.NotEmpty(t, output, "Should have captured stdout")
	assert.Contains(t, output, "1.12.5-rc1", "Should have listed a prerelease version")

}

func TestInstallPrereleases(t *testing.T) {
	_, github := createMockDVM(nil)
	defer github.Close()

	outputCapture := &bytes.Buffer{}
	color.Output = outputCapture

	dvm := makeCliApp()
	dvm.Run([]string{"dvm-helper", "--debug", "install", "1.12.5-rc1"})

	output := outputCapture.String()
	assert.NotEmpty(t, output, "Should have captured stdout")
	assert.Contains(t, output, "Now using Docker 1.12.5-rc1", "Should have installed a prerelease version")
}

func loadTestData(src string) string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	testFile := filepath.Join(pwd, "testdata", src)
	content, err := ioutil.ReadFile(testFile)
	if err != nil {
		panic(err)
	}
	return string(content)
}
