package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	config_dir_linux     = "/etc/ipv6_dns_auto/config"
	prefix_len = 64
	interfaceName_win = "Ethernet"
	interfaceName_def_linux = "ens33"
	checkInterval  = 50 * time.Second
)


func linux_setIPv6DNSServers(dnsServers []string) error {
	content, err := os.ReadFile("/etc/resolv.conf.master")
	if err != nil {
		return err
	}

	var out strings.Builder
	for i, srv := range dnsServers {
		out.WriteString("nameserver ")
		out.WriteString(srv)
		if i < len(dnsServers)-1 {
			out.WriteString("\n")
		}
	}

	replacedContent := strings.ReplaceAll(string(content), "#@ipv6_dns_servers@#", out.String())

	err = os.WriteFile("/etc/resolv.conf", []byte(replacedContent), 0644)
	if err != nil {
		return err
	}

    return nil
}


func win_setIPv6DNSServers(interfaceName string, dnsServers []string) error {
	cmdArgs := []string{"interface", "ipv6", "set", "dns", "name="+interfaceName, "source=static", "address="+dnsServers[0]}

	fmt.Println("netsh", strings.Join(cmdArgs, " "))
	cmd := exec.Command("netsh", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set IPv6 DNS server %s: %v, output: %s", dnsServers, err, string(output))
	}

	cmdArgs = []string{"interface", "ipv6", "add", "dns", "name="+interfaceName, "addr="+dnsServers[1], "index=2"}

	fmt.Println("netsh", strings.Join(cmdArgs, " "))
	cmd = exec.Command("netsh", cmdArgs...)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set IPv6 DNS server %s: %v, output: %s", dnsServers, err, string(output))
	}

	return nil
}

func setIPv6DNSServers(interfaceName string, dnsServers []string) error {

	if runtime.GOOS == "windows" {
		err := win_setIPv6DNSServers(interfaceName, dnsServers)
		if err != nil {
			return err
		}
	} else if runtime.GOOS == "linux" {
		err := linux_setIPv6DNSServers(dnsServers)
		if err != nil {
			return err
		}
	}
	return nil
}

func linux_write_prefix(prefix string) {
	// Define the directory and file paths
	dirPath := "/etc/ipv6_prefix"
	filePath := filepath.Join(dirPath, "ipv6.prefix")

	// Create the directory with appropriate permissions
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	// Create the file and open it for writing
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()
	err = os.Chmod(filePath, 0644)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}

	// Write to the file
	_, err = file.WriteString(prefix)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}

	fmt.Printf("updated prefix file to: %v\n", prefix)
}



func main() {
	var lastIPv6Prefix string = ""

	var dnsServers = []string  {
		"#@ipv6_prefix@#::@250:56ff:fe3e:d7b9",
		"#@ipv6_prefix@#::@250:56ff:fe3c:e6c4",
	}

	interfaceName := ""
	config_dir := ""

	if runtime.GOOS == "linux" {
		config_dir = config_dir_linux
		content, err := os.ReadFile(filepath.Join(config_dir, "interface"))
		if err != nil {
			interfaceName = interfaceName_def_linux
		} else {
			interfaceName = string(content)
		}
	} else if runtime.GOOS == "windows" {
		interfaceName = interfaceName_win
	}

	// Start an infinite loop
	for {

		// Get the current IPv6 prefix
		currentIPv6Prefix, err := getCurrentIPv6Prefix(interfaceName)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		// If the current prefix is different from the last one, update the zone files and reload services
		if currentIPv6Prefix != lastIPv6Prefix {

			var dnsServers_expanded []string
			for _, srv := range dnsServers {
				replacedContent := strings.ReplaceAll(srv, "#@ipv6_prefix@#::@", currentIPv6Prefix)
				dnsServers_expanded = append(dnsServers_expanded, replacedContent)
			}
			fmt.Printf("prefix: %v\ndns servers: %v\n", currentIPv6Prefix, dnsServers_expanded)
			
			err = setIPv6DNSServers(interfaceName, dnsServers_expanded)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				// fmt.Println("IPv6 DNS servers set successfully for interface:", interfaceName)
			}
			if runtime.GOOS == "linux" {
				linux_write_prefix(currentIPv6Prefix)
				reload_dns_linux()
			}

			lastIPv6Prefix = currentIPv6Prefix
		}

		// Sleep for the specified interval before checking again
		time.Sleep(checkInterval)
	}
}




// isValidIPAddress checks if an IP address is not link-local, not ULA, and not loopback.
func isValidIPAddress(ip net.IP) bool {
	if ip == nil {
		return false // Invalid IP address
	}

	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || !ip.IsGlobalUnicast() {
		return false
	}

	return true
}


// addrToIP converts a net.Addr to a net.IP if possible.
func addrToIP(addr net.Addr) (net.IP, error) {
	switch v := addr.(type) {
	case *net.IPAddr:
		return v.IP, nil
	case *net.IPNet:
		return v.IP, nil
	case *net.TCPAddr:
		return v.IP, nil
	case *net.UDPAddr:
		return v.IP, nil
	default:
		return nil, fmt.Errorf("unsupported address type: %T", addr)
	}
}

// Function to get the current IPv6 prefix
func getCurrentIPv6Prefix(interfaceName string) (string, error) {
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
	var ip net.IP
	for _, addr := range addrs {
		// Check if it's an IPv6 address and not temporary
		ip, err = addrToIP(addr)
		if err != nil {
			continue
		}
		if isValidIPAddress(ip) {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
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


func reload_dns_linux() error {
    err := exec.Command("systemctl", "reload", "NetworkManager.service").Run()
    if err != nil {
        return err
    }
    return nil
}

