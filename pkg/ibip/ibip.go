package ibip

import (
	"encoding/base64"
	"encoding/json"
	"github.com/openshift/cluster-bootstrap/pkg/common"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	ignition "github.com/coreos/ignition/v2/config/v3_1"
	ignitionTypes "github.com/coreos/ignition/v2/config/v3_1/types"
)

type ConfigIBip struct {
	AssetDir             string
	IgnitionPath      	 string
}

type iBipCommand struct {
	ignitionPath      string
	assetDir          string
}


func NewIBipCommand(config ConfigIBip) (*iBipCommand, error) {
	return &iBipCommand{
		assetDir:             config.AssetDir,
		ignitionPath:      	  config.IgnitionPath,
	}, nil
}

const (
	kubeDir                     = "/etc/kubernetes"
	assetPathBootstrapManifests = "bootstrap-manifests"
	manifests                   = "manifests"
	bootstrapConfigs            = "bootstrap-configs"
	bootstrapSecrets            = "bootstrap-secrets"
	etcdDataDir                 = "/var/lib/etcd"
)

type ignitionFile struct {
	filePath     string
	fileContents string
	mode         int
}

type filesToGather struct {
	pathForSearch string
	pattern       string
	ignitionPath  string
}

func newIgnitionFile(path string, filePathInIgnition string) (*ignitionFile, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	encodedContent := "data:text/plain;charset=utf-8;base64," + base64.StdEncoding.EncodeToString(content)
	return &ignitionFile{filePath: filePathInIgnition,
		mode: 420, fileContents: encodedContent}, nil
}

func (i *iBipCommand) createListOfIgnitionFiles(files []string, searchedFolder string, folderInIgnition string) ([]*ignitionFile, error) {
	var ignitionFiles []*ignitionFile
	for _, path := range files {
		// Take relative path
		filePath := filepath.Join(folderInIgnition, strings.ReplaceAll(path, searchedFolder, ""))
		fileToAdd, err := newIgnitionFile(path, filePath)
		if err != nil {
			common.UserOutput("Failed to read %s", path)
			return nil, err
		}
		ignitionFiles = append(ignitionFiles, fileToAdd)
	}
	return ignitionFiles, nil
}

func (i *iBipCommand) createFilesList(filesToGatherList []filesToGather) ([]*ignitionFile, error) {
	var fullList []*ignitionFile
	for _, ft := range filesToGatherList {
		files, err := i.findFiles(ft.pathForSearch, ft.pattern)
		if err != nil {
			common.UserOutput("Failed to search for files in %s with pattern %s, err %e", ft.pathForSearch, ft.pattern, err)
			return nil, err
		}
		ignitionFiles, err := i.createListOfIgnitionFiles(files, ft.pathForSearch, ft.ignitionPath)
		if err != nil {
			common.UserOutput("Failed to create ignitionsFile list for in %s with ign path %s, err %e", ft.pathForSearch, ft.ignitionPath, err)
			return nil, err
		}
		fullList = append(fullList, ignitionFiles...)
	}
	return fullList, nil
}

func (i *iBipCommand) UpdateSnoIgnitionData() error {

	common.UserOutput("Creating ignition file objects from required folders")
	filesFromFolders := []filesToGather{
		{pathForSearch: filepath.Join(i.assetDir, assetPathBootstrapManifests), pattern: "kube*", ignitionPath: filepath.Join(kubeDir, manifests)},
		{pathForSearch: filepath.Join(kubeDir, bootstrapConfigs), pattern: "*", ignitionPath: filepath.Join(kubeDir, bootstrapConfigs)},
		{pathForSearch: filepath.Join(i.assetDir, "tls"), pattern: "*", ignitionPath: filepath.Join(kubeDir, bootstrapSecrets)},
		{pathForSearch: filepath.Join(i.assetDir, "etcd-bootstrap/bootstrap-manifests/secrets"), pattern: "*", ignitionPath: filepath.Join(kubeDir, "static-pod-resources/etcd-member")},
		{pathForSearch: etcdDataDir, pattern: "*", ignitionPath: etcdDataDir},
	}

	ignitionFileObjects, err := i.createFilesList(filesFromFolders)
	if err != nil {
		return err
	}

	common.UserOutput("Creating ignition file objects from files that require rename")
	singleFilesWithNameChange := map[string]string{
		filepath.Join(i.assetDir, "auth/kubeconfig-loopback"):                                filepath.Join(kubeDir, bootstrapSecrets+"/kubeconfig"),
		filepath.Join(i.assetDir, "tls/etcd-ca-bundle.crt"):                                  filepath.Join(kubeDir, "static-pod-resources/etcd-member/ca.crt"),
		filepath.Join(i.assetDir, "etcd-bootstrap/bootstrap-manifests/etcd-member-pod.yaml"): filepath.Join(kubeDir, manifests+"/etcd-pod.yaml"),
	}

	for path, ignPath := range singleFilesWithNameChange {
		fileToAdd, err := newIgnitionFile(path, ignPath)
		if err != nil {
			common.UserOutput("Error occurred while trying to create ignitionFile from %s with ign path %s, err : %e", path, ignPath, err)
			return err
		}
		ignitionFileObjects = append(ignitionFileObjects, fileToAdd)
	}

	common.UserOutput("Ignition Path %s", i.ignitionPath)
	err = i.addFilesToIgnitionFile(i.ignitionPath, ignitionFileObjects)
	if err != nil {
		common.UserOutput("Error occurred while trying to read %s : %e", i.ignitionPath, err)
		return err
	}

	return nil
}

func (i *iBipCommand) addFilesToIgnitionObject(ignitionData []byte, files []*ignitionFile) ([]byte, error) {

	ignitionOutput, _, err := ignition.Parse(ignitionData)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		common.UserOutput("Adding file %s", file.filePath)
		rootUser := "root"
		iFile := ignitionTypes.File{
			Node: ignitionTypes.Node{
				Path:      file.filePath,
				Overwrite: nil,
				Group:     ignitionTypes.NodeGroup{},
				User:      ignitionTypes.NodeUser{Name: &rootUser},
			},
			FileEmbedded1: ignitionTypes.FileEmbedded1{
				Append: []ignitionTypes.Resource{},
				Contents: ignitionTypes.Resource{
					Source: &file.fileContents,
				},
				Mode: &file.mode,
			},
		}
		ignitionOutput.Storage.Files = append(ignitionOutput.Storage.Files, iFile)
	}
	return json.Marshal(ignitionOutput)
}

func (i *iBipCommand) addFilesToIgnitionFile(ignitionPath string, files []*ignitionFile) error {
	common.UserOutput("Adding files %d to ignition %s", len(files), ignitionPath)
	ignitionData, err := ioutil.ReadFile(ignitionPath)
	if err != nil {
		common.UserOutput("Error occurred while trying to read %s : %e", ignitionPath, err)
		return err
	}
	newIgnitionData, err := i.addFilesToIgnitionObject(ignitionData, files)
	if err != nil {
		common.UserOutput("Failed to write new ignition to %s : %e", ignitionPath, err)
		return err
	}

	err = ioutil.WriteFile(ignitionPath, newIgnitionData, os.ModePerm)
	if err != nil {
		common.UserOutput("Failed to write new ignition to %s", ignitionPath)
		return err
	}

	return nil
}

func (i *iBipCommand) findFiles(root string, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}