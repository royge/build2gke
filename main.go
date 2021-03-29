package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	ip, err := getIPAddress()
	if err != nil {
		log.Fatalf("unable to get IP address: %v", err)
		return
	}

	log.Println("IP Address:", ip)

	if err := updateGKEAuthorizedNetwork(ip); err != nil {
		log.Fatalf("unable to update GKE cluster authorized network: %v", err)
		return
	}

	log.Println("DONE.")
}

func getIPAddress() (string, error) {
	url := "https://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip"
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("unable to query ip address from metadata URL: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func updateGKEAuthorizedNetwork(ipAddress string) error {
	return errors.New("TODO")
}
