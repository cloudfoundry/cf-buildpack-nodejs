package hooks_test

import (
	"bytes"
	"errors"
	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/nodejs-buildpack/src/nodejs/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var _ = Describe("seekerHook", func() {
	var (
		err      error
		buildDir string
		depsDir  string
		depsIdx  string
		logger   *libbuildpack.Logger
		stager   *libbuildpack.Stager
		buffer   *bytes.Buffer
		seeker   hooks.SeekerAfterCompileHook
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "nodejs-buildpack.build.")
		Expect(err).To(BeNil())

		depsDir, err = ioutil.TempDir("", "nodejs-buildpack.deps.")
		Expect(err).To(BeNil())

		depsIdx = "07"
		err = os.MkdirAll(filepath.Join(depsDir, depsIdx), 0755)

		buffer = new(bytes.Buffer)
		logger = libbuildpack.NewLogger(buffer)

		logger := libbuildpack.NewLogger(os.Stdout)
		command := &libbuildpack.Command{}

		seeker = hooks.SeekerAfterCompileHook{
			Command:    command,
			Log:        logger,
			Downloader: hooks.SeekerDownloader{},
			Unzzipper:  hooks.SeekerUnzipper{},
		}
	})

	JustBeforeEach(func() {
		args := []string{buildDir, "", depsDir, depsIdx}
		stager = libbuildpack.NewStager(args, logger, &libbuildpack.Manifest{})
	})

	AfterEach(func() {

		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(depsDir)
		Expect(err).To(BeNil())
		os.RemoveAll(filepath.Join(os.TempDir(), "seeker_tmp"))
	})
	Describe("AfterCompile - obtain agent by extracting from sensor", func() {
		var (
			oldVcapApplication string
			oldVcapServices    string
			oldBpDebug         string
		)
		BeforeEach(func() {
			oldVcapApplication = os.Getenv("VCAP_APPLICATION")
			oldVcapServices = os.Getenv("VCAP_SERVICES")
			oldBpDebug = os.Getenv("BP_DEBUG")
			seeker.Downloader = getMockedSensorDownloader()
			seeker.Unzzipper = getSensorUnzipper()
			seeker.Versioner = getMockedVersioner("2018.05-SP1")
		})
		AfterEach(func() {
			os.Setenv("VCAP_APPLICATION", oldVcapApplication)
			os.Setenv("VCAP_SERVICES", oldVcapServices)
			os.Setenv("BP_DEBUG", oldBpDebug)
		})

		Context("VCAP_SERVICES contains seeker service - as a user provided service", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", `{"name":"pcf app"}`)
				os.Setenv("VCAP_SERVICES", `{
											 "user-provided": [
											   {
											     "name": "seeker_service_v2",
											     "instance_name": "seeker_service_v2",
											     "binding_name": null,
											     "credentials": {
													"seeker_server_url": "http://10.120.9.117:9911",
											       "enterprise_server_url": "http://10.120.9.117:8082",
											       "sensor_host": "localhost",
											       "sensor_port": "9911"
											     },
											     "syslog_drain_url": "",
											     "volume_mounts": [],
											     "label": "user-provided",
											     "tags": []
											   }
											 ]
											}`)
			})
			It("installs seeker", func() {
				err = seeker.AfterCompile(stager)
				Expect(err).To(BeNil())

				// Sets up profile.d
				envFile := filepath.Join(depsDir, depsIdx, "profile.d", "seeker-env.sh")
				contents, err := ioutil.ReadFile(envFile)
				Expect(err).To(BeNil())

				expected := "\n" +
					"export SEEKER_SENSOR_HOST=localhost\n" +
					"export SEEKER_SENSOR_HTTP_PORT=9911\n" +
					"export SEEKER_SERVER_URL=http://10.120.9.117:9911\n"
				Expect(string(contents)).To(Equal(expected))
			})
		})
		Context("VCAP_SERVICES contains seeker service - as a regular service", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", `{"name":"pcf app"}`)
				os.Setenv("VCAP_SERVICES", `{
																			"seeker-security-service": [
																			 {
																			   "name": "seeker_instace",
																			   "instance_name": "seeker_instace",
																			   "binding_name": null,
																			   "credentials": {
																			     "sensor_host": "localhost",
																			     "sensor_port": "9911",
																				"seeker_server_url": "http://10.120.9.117:9911",
											       								"enterprise_server_url": "http://10.120.9.117:8082"
						
																			   },
																			   "syslog_drain_url": null,
																			   "volume_mounts": [],
																			   "label": null,
																			   "provider": null,
																			   "plan": "default-seeker-plan-new",
																			   "tags": [
																			     "security",
																			     "agent",
																			     "monitoring"
																			   ]
																			 }
																			],
																			"2": [{"name":"mysql"}]}
																			`)

			})
			It("installs seeker", func() {
				err = seeker.AfterCompile(stager)
				Expect(err).To(BeNil())

				// Sets up profile.d
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "seeker-env.sh"))
				Expect(err).To(BeNil())
				expected := "\n" +
					"export SEEKER_SENSOR_HOST=localhost\n" +
					"export SEEKER_SENSOR_HTTP_PORT=9911\n" +
					"export SEEKER_SERVER_URL=http://10.120.9.117:9911\n"
				Expect(string(contents)).To(Equal(expected))
			})

		})
	})
	Describe("AfterCompile - obtain agent by directly downloading the agent", func() {
		var (
			oldVcapApplication string
			oldVcapServices    string
			oldBpDebug         string
		)
		BeforeEach(func() {
			oldVcapApplication = os.Getenv("VCAP_APPLICATION")
			oldVcapServices = os.Getenv("VCAP_SERVICES")
			oldBpDebug = os.Getenv("BP_DEBUG")
			seeker.Versioner = getMockedVersioner("2018.06-SNAPSHOT")
			seeker.Downloader = getMockedAgentDownloader()
			seeker.Unzzipper = getAgentUnzipper()
		})
		AfterEach(func() {

			os.Setenv("VCAP_APPLICATION", oldVcapApplication)
			os.Setenv("VCAP_SERVICES", oldVcapServices)
			os.Setenv("BP_DEBUG", oldBpDebug)
		})

		Context("VCAP_SERVICES contains seeker service - as a user provided service", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", `{"name":"pcf app"}`)
				os.Setenv("VCAP_SERVICES", `{
																			 "user-provided": [
																			   {
																			     "name": "seeker_service_v2",
																			     "instance_name": "seeker_service_v2",
																			     "binding_name": null,
																			     "credentials": {
																					"seeker_server_url": "http://10.120.9.117:9911",
																			       "enterprise_server_url": "http://10.120.9.117:8082",
																			       "sensor_host": "localhost",
																			       "sensor_port": "9911"
																			     },
																			     "syslog_drain_url": "",
																			     "volume_mounts": [],
																			     "label": "user-provided",
																			     "tags": []
																			   }
																			 ]
																			}`)
			})
			It("installs seeker", func() {
				err = seeker.AfterCompile(stager)
				Expect(err).To(BeNil())

				// Sets up profile.d
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "seeker-env.sh"))
				Expect(err).To(BeNil())

				expected := "\n" +
					"export SEEKER_SENSOR_HOST=localhost\n" +
					"export SEEKER_SENSOR_HTTP_PORT=9911\n" +
					"export SEEKER_SERVER_URL=http://10.120.9.117:9911\n"
				Expect(string(contents)).To(Equal(expected))
			})
		})
		Context("VCAP_SERVICES contains seeker service - as a regular service", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", `{"name":"pcf app"}`)
				os.Setenv("VCAP_SERVICES", `{
																			"seeker-security-service": [
																			 {
																			   "name": "seeker_instance",
																			   "instance_name": "seeker_instance",
																			   "binding_name": null,
																			   "credentials": {
																			     "sensor_host": "localhost",
																			     "sensor_port": "9911",
																				"seeker_server_url": "http://10.120.9.117:9911",
																		       "enterprise_server_url": "http://10.120.9.117:8082"
						
																			   },
																			   "syslog_drain_url": null,
																			   "volume_mounts": [],
																			   "label": null,
																			   "provider": null,
																			   "plan": "default-seeker-plan-new",
																			   "tags": [
																			     "security",
																			     "agent",
																			     "monitoring"
																			   ]
																			 }
																			],
																			"2": [{"name":"mysql"}]}
																			`)

			})
			It("installs seeker", func() {
				err = seeker.AfterCompile(stager)
				Expect(err).To(BeNil())

				// Sets up profile.d
				contents, err := ioutil.ReadFile(filepath.Join(depsDir, depsIdx, "profile.d", "seeker-env.sh"))
				Expect(err).To(BeNil())

				expected := "\n" +
					"export SEEKER_SENSOR_HOST=localhost\n" +
					"export SEEKER_SENSOR_HTTP_PORT=9911\n" +
					"export SEEKER_SERVER_URL=http://10.120.9.117:9911\n"
				Expect(string(contents)).To(Equal(expected))
			})

		})

	})
	Describe("AfterCompile - agent download choosing strategy", func() {
		var (
			oldVcapApplication string
			oldVcapServices    string
			oldBpDebug         string
		)
		BeforeEach(func() {
			oldVcapApplication = os.Getenv("VCAP_APPLICATION")
			oldVcapServices = os.Getenv("VCAP_SERVICES")
			oldBpDebug = os.Getenv("BP_DEBUG")
			os.Unsetenv("SEEKER_APP_ENTRY_POINT")
		})
		AfterEach(func() {
			os.Setenv("VCAP_APPLICATION", oldVcapApplication)
			os.Setenv("VCAP_SERVICES", oldVcapServices)
			os.Setenv("BP_DEBUG", oldBpDebug)
		})

		Context("VCAP_SERVICES contains seeker service - as a user provided service", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", `{"name":"pcf app"}`)
				os.Setenv("VCAP_SERVICES", `{
																		  "user-provided": [
																		    {
																		      "name": "seeker_service_v2",
																		      "instance_name": "seeker_service_v2",
																		      "binding_name": null,
																		      "credentials": {
																				"seeker_server_url": "http://10.120.9.117:9911",
											       								"enterprise_server_url": "http://10.120.9.117:8082",
																		        "sensor_host": "localhost",
																		        "sensor_port": "9911"
																		      },
																		      "syslog_drain_url": "",
																		      "volume_mounts": [],
																		      "label": "user-provided",
																		      "tags": []
																		    }
																		  ]
																		 }`)
			})
			It("Chooses downloading the sensor for Seeker versions older than 2018.05 (including 2018.05)", func() {
				seeker.Downloader = getMockedSensorDownloader()
				seeker.Unzzipper = getSensorUnzipper()

				seekerVersionSupportingSensorDownloadOnly := []string{"2018.05", "2018.04", "2018.03", "2018.02", "2018.01", "2017.12", "2017.11", "2017.10", "2017.09", "2017.08", "2017.05", "2017.04", "2017.03", "2017.02", "2017.01"}
				for _, seekerVersion := range seekerVersionSupportingSensorDownloadOnly {
					seeker.Versioner = getMockedVersioner(seekerVersion)
					err = seeker.AfterCompile(stager)
					Expect(err).To(BeNil())
				}

			})
			It("Chooses downloading the agent for Seeker versions newer than 2018.05", func() {
				seeker.Downloader = getMockedAgentDownloader()
				seeker.Unzzipper = getAgentUnzipper()
				seekerVersionSupportingAgentDownload := []string{"2018.06", "2018.07", "2018.08", "2018.09", "2018.10", "2018.11", "2018.12", "2019.01", "2019.02", "2019.03", "2019.04", "2019.05"}
				for _, seekerVersion := range seekerVersionSupportingAgentDownload {
					seeker.Versioner = getMockedVersioner(seekerVersion)
					err = seeker.AfterCompile(stager)
					Expect(err).To(BeNil())
				}
			})

		})
	})
	Describe("AfterCompile - adding agent require code to entry point", func() {
		var (
			oldVcapApplication string
			oldVcapServices    string
			oldBpDebug         string
		)
		BeforeEach(func() {
			oldVcapApplication = os.Getenv("VCAP_APPLICATION")
			oldVcapServices = os.Getenv("VCAP_SERVICES")
			oldBpDebug = os.Getenv("BP_DEBUG")
			os.Setenv("SEEKER_APP_ENTRY_POINT", "server.js")
		})
		AfterEach(func() {
			os.Setenv("VCAP_APPLICATION", oldVcapApplication)
			os.Setenv("VCAP_SERVICES", oldVcapServices)
			os.Setenv("BP_DEBUG", oldBpDebug)
		})

		Context("VCAP_SERVICES contains seeker service - as a user provided service", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", `{"name":"pcf app"}`)
				os.Setenv("VCAP_SERVICES", `{
																										  "user-provided": [
																										    {
																										      "name": "seeker_service_v2",
																										      "instance_name": "seeker_service_v2",
																										      "binding_name": null,
																										      "credentials": {
																												"seeker_server_url": "http://10.120.9.117:9911",
																										       "enterprise_server_url": "http://10.120.9.117:8082",
																										        "sensor_host": "localhost",
																										        "sensor_port": "9911"
																										      },
																										      "syslog_drain_url": "",
																										      "volume_mounts": [],
																										      "label": "user-provided",
																										      "tags": []
																										    }
																										  ]
																										 }`)
			})
			It("Prepends 'require('./seeker/node_modules/@synopsys-sig/seeker);' to the server.js file for version newer than 2019.02", func() { //fail
				seeker.Versioner = getMockedVersioner("2019.03")
				seeker.Downloader = getMockedAgentDownloader()
				seeker.Unzzipper = getAgentUnzipper()
				entryPointPath := filepath.Join(buildDir, "server.js")
				Expect(entryPointPath).ToNot(BeAnExistingFile())
				const mockedCode = "some mock javascript code"
				Expect(ioutil.WriteFile(entryPointPath, []byte(mockedCode), 0755)).To(Succeed())
				err = seeker.AfterCompile(stager)
				Expect(err).To(BeNil())
				contents, err := ioutil.ReadFile(entryPointPath)
				Expect(err).To(BeNil())
				Expect(string(contents)).To(Equal(
					"require('./seeker/node_modules/@synopsys-sig/seeker');\n" +
						mockedCode + "\n"))
			})
			It("Prepends 'require('./seeker/node_modules/@synopsys-sig/seeker-inline');' to the server.js file for version older than 2019.02", func() { //fail
				seeker.Versioner = getMockedVersioner("2019.01")
				seeker.Downloader = getMockedAgentDownloader()
				seeker.Unzzipper = getAgentUnzipper()
				entryPointPath := filepath.Join(buildDir, "server.js")
				Expect(entryPointPath).ToNot(BeAnExistingFile())
				const mockedCode = "some mock javascript code"
				Expect(ioutil.WriteFile(entryPointPath, []byte(mockedCode), 0755)).To(Succeed())
				err = seeker.AfterCompile(stager)
				Expect(err).To(BeNil())
				contents, err := ioutil.ReadFile(entryPointPath)
				Expect(err).To(BeNil())
				Expect(string(contents)).To(Equal(
					"require('./seeker/node_modules/@synopsys-sig/seeker-inline');\n" +
						mockedCode + "\n"))
			})
			It("Fails to prepend the require to the server.js file - when the file does not exist", func() {
				seeker.Versioner = MockVersioner{Mock: func(credentials hooks.SeekerCredentials) (e error, s string) {
					return nil, "2019.01"
				}}
				entryPointPath := filepath.Join(buildDir, "server.js")
				Expect(entryPointPath).ToNot(BeAnExistingFile())
				err = seeker.AfterCompile(stager)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("no such file or directory"))
			})

		})
	})
})

func getMockedAgentDownloader() MockDownloader {
	return MockDownloader{Mock: func(url, path string) error {
		const expectedUrlSuffix = "rest/api/latest/installers/agents/binaries/NODEJS"
		if !strings.HasSuffix(url, expectedUrlSuffix) {
			return errors.New("expected to be called with url that ends with " + expectedUrlSuffix)
		}
		return nil
	}}
}

func getMockedSensorDownloader() MockDownloader {
	return MockDownloader{Mock: func(url, path string) error {
		const expectedUrlSuffix = "rest/ui/installers/binaries/LINUX"
		if !strings.HasSuffix(url, expectedUrlSuffix) {
			return errors.New("expected to be called with url that ends with " + expectedUrlSuffix)
		}
		return nil
	}}
}

func getMockedVersioner(versionToMock string) MockVersioner {
	return MockVersioner{Mock: func(credentials hooks.SeekerCredentials) (e error, s string) {
		return nil, versionToMock
	}}
}

func getSensorUnzipper() MockUnzipper {
	return MockUnzipper{Mock: func(zipFile, absoluteFolderPath string) error {
		s, e := getFixtureAbsolutePath("NODEJS_SensorWithAgent.zip")
		if e != nil {
			return e
		}
		z := hooks.SeekerUnzipper{Command: &libbuildpack.Command{}}
		return z.Unzip(s, absoluteFolderPath)
	}}
}
func getAgentUnzipper() MockUnzipper {
	return MockUnzipper{Mock: func(zipFile, absoluteFolderPath string) error {
		s, e := getFixtureAbsolutePath("NODEJS_agent.zip")
		if e != nil {
			return e
		}
		z := hooks.SeekerUnzipper{Command: &libbuildpack.Command{}}
		return z.Unzip(s, absoluteFolderPath)
	}}
}

func getFixtureAbsolutePath(fileName string) (string, error) {
	path, err := filepath.Abs("../../../fixtures/seeker/" + fileName)
	return path, err
}

type MockDownloader struct {
	Mock func(url, path string) error
}

func (f MockDownloader) DownloadFile(url, path string) error {
	return f.Mock(url, path)
}

type MockUnzipper struct {
	Mock func(zipFile, absoluteFolderPath string) error
}

func (u MockUnzipper) Unzip(zipFile, absoluteFolderPath string) error {
	return u.Mock(zipFile, absoluteFolderPath)
}

type MockVersioner struct {
	Mock func(credentials hooks.SeekerCredentials) (error, string)
}

func (m MockVersioner) GetSeekerVersion(credentials hooks.SeekerCredentials) (error, string) {
	return m.Mock(credentials)
}
