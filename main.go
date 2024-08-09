package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Package struct {
	Name     string   `json:"name"`
	Version  string   `json:"ver"`
	Files    []string `json:"targets"`
	Packages []struct {
		Name    string `json:"name"`
		Version string `json:"ver"`
	} `json:"packages"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: pm <command> <file>")
		return
	}

	command := os.Args[1]
	file := os.Args[2]

	switch command {
	case "create":
		createPackage(file)
	case "update":
		updatePackage(file)
	default:
		fmt.Println("Unknown command")
	}
}

func createPackage(jsonFile string) {
	// Read JSON file
	data, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	var pkg Package
	err = json.Unmarshal(data, &pkg)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Create ZIP archive
	zipFile := pkg.Name + "_" + pkg.Version + ".zip"
	archive, err := os.Create(zipFile)
	if err != nil {
		fmt.Println("Error creating ZIP file:", err)
		return
	}

	zipWriter := zip.NewWriter(archive)

	// Add files to ZIP
	for _, file := range pkg.Files {
		err = addFileToZip(zipWriter, file)
		if err != nil {
			fmt.Println("Error adding file to ZIP:", err)
			zipWriter.Close()
			archive.Close()
			return
		}
	}

	// Close the zip writer before closing the file
	err = zipWriter.Close()
	if err != nil {
		fmt.Println("Error closing ZIP writer:", err)
		archive.Close()
		return
	}

	// Close the archive file
	err = archive.Close()
	if err != nil {
		fmt.Println("Error closing archive file:", err)
		return
	}

	fmt.Println("Package created:", zipFile)

	// Upload to server
	err = uploadToServer(zipFile)
	if err != nil {
		fmt.Println("Error uploading to server:", err)
		return
	}

	fmt.Println("Package uploaded to server")
}

func updatePackage(jsonFile string) {
	// Read JSON file
	data, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	var pkg Package
	err = json.Unmarshal(data, &pkg)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// Download from server

	for _, file := range pkg.Packages {
		zipFile := file.Name + "_" + file.Version + ".zip"
		err = downloadFromServer(zipFile)
		if err != nil {
			fmt.Println("Error downloading from server:", err)
			return
		}

		fmt.Println("Package downloaded from server")
		// Unzip package
		err = unzipPackage(zipFile)
		if err != nil {
			fmt.Println("Error unzipping package:", err)
			return
		}
	}

	fmt.Println("Package updated successfully")
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filename

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func uploadToServer(filename string) error {
	// SSH connection details
	sshConfig := &ssh.ClientConfig{
		User: "lainlynr",
		Auth: []ssh.AuthMethod{
			ssh.Password("1233321"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to the SSH server
	conn, err := ssh.Dial("tcp", "127.0.0.1:22", sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %v", err)
	}
	defer conn.Close()

	// Create SFTP client
	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close()

	// Open local file
	localFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close()

	// Create remote file
	remoteFile, err := sftpClient.Create("." + filename)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	// Copy file contents
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %v", err)
	}

	// Explicitly close files
	localFile.Close()
	remoteFile.Close()

	return nil
}

func downloadFromServer(filename string) error {
	// SSH connection details
	sshConfig := &ssh.ClientConfig{
		User: "lainlynr",
		Auth: []ssh.AuthMethod{
			ssh.Password("1233321"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to the SSH server
	conn, err := ssh.Dial("tcp", "127.0.0.1:22", sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %v", err)
	}
	defer conn.Close()

	// Create SFTP client
	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close()

	// Open remote file
	remoteFile, err := sftpClient.Open("." + filename)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v, %s", err, filename)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()

	// Copy file contents
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %v", err)
	}

	return nil
}

func unzipPackage(zipFile string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %v", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(".", file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in zip: %v", err)
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create target file: %v", err)
		}
		defer targetFile.Close()

		_, err = io.Copy(targetFile, fileReader)
		if err != nil {
			return fmt.Errorf("failed to copy file contents: %v", err)
		}
	}

	return nil
}
