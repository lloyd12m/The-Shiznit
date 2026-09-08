package utils

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

var (
	FileLock sync.Mutex
)

// RemoveFromFile removes a specific line from a file
func RemoveFromFile(filename, lineToRemove string) error {
	FileLock.Lock()
	defer FileLock.Unlock()

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != lineToRemove && line != "" {
			lines = append(lines, line)
		}
	}

	tempFile, err := os.Create(filename + ".tmp")
	if err != nil {
		return err
	}
	defer tempFile.Close()

	writer := bufio.NewWriter(tempFile)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}
	writer.Flush()
	tempFile.Close()
	file.Close()

	return os.Rename(filename+".tmp", filename)
}

// SaveClaim saves the successful claim to claims.txt
func SaveClaim(target, session string) error {
	FileLock.Lock()
	defer FileLock.Unlock()

	file, err := os.OpenFile("claims.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s: %s\n", target, session)
	return err
}

// ReadFileLines reads all lines from a file
func ReadFileLines(filename string) ([]string, error) {
	FileLock.Lock()
	defer FileLock.Unlock()

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
