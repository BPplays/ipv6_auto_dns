package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

const (
    configFile     = "/etc/bind/.ipv6_prefix"
	prefix_len = 64
	interfaceName = "ens33"
    checkInterval  = 50 * time.Second
)





func setIPv6DNSServers(interfaceName string, dnsServers string) error {

    cmdArgs := []string{"/SetDNS6", dnsServers, interfaceName}

    cmd := exec.Command("QuickSetDNS.exe", cmdArgs...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to set IPv6 DNS server %s: %v, output: %s", dnsServers, err, string(output))
    }

	return nil
}




func main() {
    var lastIPv6Prefix string = ""


    var dnsServers = []string  {
        "#@ipv6_prefix@#::@250:56ff:fe3e:d7b9",
        "#@ipv6_prefix@#::@250:56ff:fe3c:e6c4",
    }

    dnsServers_join := strings.Join(dnsServers, ",")

    // Start an infinite loop
    for {

        // Get the current IPv6 prefix
        currentIPv6Prefix, err := getCurrentIPv6Prefix()
        if err != nil {
            fmt.Println("Error:", err)
            return
        }

        // If the current prefix is different from the last one, update the zone files and reload services
        if currentIPv6Prefix != lastIPv6Prefix {
			fmt.Printf("prefix: %v\n", currentIPv6Prefix)


            dnsServers_expanded := strings.ReplaceAll(string(dnsServers_join), "#@ipv6_prefix@#::@", currentIPv6Prefix)
        
            err = setIPv6DNSServers(interfaceName, dnsServers_expanded)
            if err != nil {
                fmt.Printf("Error: %v\n", err)
            } else {
                fmt.Println("IPv6 DNS servers set successfully for interface:", interfaceName)
            }

            lastIPv6Prefix = currentIPv6Prefix
        }

        // Sleep for the specified interval before checking again
        time.Sleep(checkInterval)
    }
}






// Function to get the current IPv6 prefix
func getCurrentIPv6Prefix() (string, error) {
    // Specify the network interface name
    // interfaceName := "eth0" // Change this to your desired interface name

    // Get network interface
    iface, err := net.InterfaceByName(interfaceName)
    if err != nil {
        return "", err
    }

    // Get addresses for the interface
    addrs, err := iface.Addrs()
    if err != nil {
        return "", err
    }

    // Initialize variables to store the IPv6 prefix
    var ipv6Prefix string

    // Iterate over addresses to find the IPv6 prefix
    for _, addr := range addrs {
        // Check if it's an IPv6 address and not temporary
        if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() == nil && !ipnet.IP.IsLinkLocalUnicast() {
            ipv6Prefix = getIPv6Prefix(ipnet)
            break
        }
    }

    // If no IPv6 prefix found, return an error
    if ipv6Prefix == "" {
        return "", fmt.Errorf("no IPv6 prefix found")
    }

    return ipv6Prefix, nil
}

// Function to extract the IPv6 prefix from an IPNet object and pad it to /64 length
func getIPv6Prefix(ipnet *net.IPNet) string {
    // Get the network portion of the IP
    network := ipnet.IP.Mask(ipnet.Mask)

    // Convert the network portion to a string representation
    ipv6Prefix := network.String()

    // If the prefix length is less than 64, pad it with zeros
    if len(ipv6Prefix) < len("xxxx:xxxx:xxxx:xxxx") {
        ipv6Prefix = strings.TrimSuffix(ipv6Prefix, ":") // Remove trailing ":"
        padding := "0000:0000:0000:0000:0000:0000:0000:"   // Pad with zeros
        ipv6Prefix += padding[len(ipv6Prefix):]          // Add padding to reach /64 length
    }

    // Ensure it ends with a single colon
    if !strings.HasSuffix(ipv6Prefix, ":") {
        ipv6Prefix += ":"
    }

    // Remove one colon until the character before the last is not a colon
    for strings.HasSuffix(ipv6Prefix, "::") {
        ipv6Prefix = strings.TrimSuffix(ipv6Prefix, ":")
    }

    return ipv6Prefix
}


