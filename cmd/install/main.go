package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// ANSI color codes for that classic installer look
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

type Installer struct {
	installDir string
	platform   string
	arch       string
}

func main() {
	installer := &Installer{
		platform: runtime.GOOS,
		arch:     runtime.GOARCH,
	}

	installer.showBanner()
	installer.checkSystem()
	installer.getInstallDir()
	installer.confirmInstall()
	installer.performInstall()
	installer.showCompletion()
}

func (i *Installer) showBanner() {
	fmt.Print(colorCyan + colorBold)
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                       ViSiON/3 BBS INSTALLER                     ║")
	fmt.Println("║                                                                  ║")
	fmt.Println("║             Modern Recreation of Classic BBS Software            ║")
	fmt.Println("║                                                                  ║")
	fmt.Println("║                      Version 1.0 - September 2025                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Print(colorReset)
	fmt.Println()
	
	fmt.Printf("%sDetected Platform:%s %s/%s\n", colorYellow, colorReset, i.platform, i.arch)
	fmt.Println()
	
	time.Sleep(1 * time.Second)
}

func (i *Installer) checkSystem() {
	fmt.Printf("%s[CHECKING SYSTEM]%s\n", colorBlue+colorBold, colorReset)
	
	var criticalMissing []string
	var optionalMissing []string
	
	// Check for ssh-keygen (required for BBS functionality)
	fmt.Print("Checking for ssh-keygen... ")
	if _, err := exec.LookPath("ssh-keygen"); err == nil {
		fmt.Printf("%s✓ ssh-keygen found%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("%s✗ ssh-keygen not found%s\n", colorRed, colorReset)
		criticalMissing = append(criticalMissing, "ssh-keygen")
	}
	
	// Check for Go (optional, we include binaries)
	fmt.Print("Checking for Go... ")
	if _, err := exec.LookPath("go"); err == nil {
		fmt.Printf("%s✓ Go found%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("%s✗ Go not found (using precompiled binaries)%s\n", colorYellow, colorReset)
	}
	
	// Check for ZMODEM tools (optional but recommended)
	fmt.Print("Checking for ZMODEM tools... ")
	szFound := false
	rzFound := false
	
	if _, err := exec.LookPath("sz"); err == nil {
		szFound = true
	}
	if _, err := exec.LookPath("rz"); err == nil {
		rzFound = true
	}
	
	if szFound && rzFound {
		fmt.Printf("%s✓ ZMODEM tools found%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("%s✗ ZMODEM tools not found%s\n", colorYellow, colorReset)
		optionalMissing = append(optionalMissing, "lrzsz (sz/rz)")
	}
	
	// Bail out if critical dependencies are missing
	if len(criticalMissing) > 0 {
		fmt.Println()
		fmt.Printf("%s[CRITICAL ERROR]%s\n", colorRed+colorBold, colorReset)
		fmt.Printf("%sThe following required dependencies are missing:%s\n", colorRed, colorReset)
		for _, dep := range criticalMissing {
			fmt.Printf("  • %s\n", dep)
		}
		fmt.Println()
		fmt.Printf("%sInstallation instructions:%s\n", colorYellow, colorReset)
		
		switch i.platform {
		case "darwin":
			fmt.Printf("  %sbrew install openssh%s\n", colorCyan, colorReset)
		case "linux":
			fmt.Printf("  %ssudo apt-get install openssh-client%s (Debian/Ubuntu)\n", colorCyan, colorReset)
			fmt.Printf("  %ssudo yum install openssh-clients%s (RHEL/CentOS)\n", colorCyan, colorReset)
			fmt.Printf("  %ssudo dnf install openssh-clients%s (Fedora)\n", colorCyan, colorReset)
		case "windows":
			fmt.Printf("  Install OpenSSH or use WSL\n")
		}
		
		fmt.Printf("%sPlease install the required dependencies and run the installer again.%s\n", colorRed, colorReset)
		os.Exit(1)
	}
	
	// Show optional dependencies
	if len(optionalMissing) > 0 {
		fmt.Println()
		fmt.Printf("%sOptional dependencies missing (install for full functionality):%s\n", colorYellow, colorReset)
		for _, dep := range optionalMissing {
			fmt.Printf("  • %s\n", dep)
		}
		fmt.Println()
		fmt.Printf("%sTo install lrzsz:%s\n", colorYellow, colorReset)
		switch i.platform {
		case "darwin":
			fmt.Printf("  %sbrew install lrzsz%s\n", colorCyan, colorReset)
		case "linux":
			fmt.Printf("  %ssudo apt-get install lrzsz%s (Debian/Ubuntu)\n", colorCyan, colorReset)
			fmt.Printf("  %ssudo yum install lrzsz%s (RHEL/CentOS)\n", colorCyan, colorReset)
			fmt.Printf("  %ssudo dnf install lrzsz%s (Fedora)\n", colorCyan, colorReset)
		case "windows":
			fmt.Printf("  Download lrzsz for Windows or use WSL\n")
		}
	}
	
	fmt.Println()
	time.Sleep(1 * time.Second)
}

func (i *Installer) getInstallDir() {
	fmt.Printf("%s[INSTALLATION DIRECTORY]%s\n", colorBlue+colorBold, colorReset)
	
	// Default installation directory based on platform
	var defaultDir string
	switch i.platform {
	case "windows":
		defaultDir = "C:\\ViSiON3"
	case "darwin":
		defaultDir = "/usr/local/vision3"
	default: // Linux and others
		defaultDir = "/opt/vision3"
	}
	
	fmt.Printf("Default installation directory: %s%s%s\n", colorCyan, defaultDir, colorReset)
	fmt.Printf("Press ENTER to accept, or type a new path: ")
	
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	
	if input == "" {
		i.installDir = defaultDir
	} else {
		i.installDir = input
	}
	
	fmt.Printf("Installation directory set to: %s%s%s\n", colorGreen, i.installDir, colorReset)
	fmt.Println()
}

func (i *Installer) confirmInstall() {
	fmt.Printf("%s[INSTALLATION CONFIRMATION]%s\n", colorBlue+colorBold, colorReset)
	fmt.Println("The following will be installed:")
	fmt.Printf("  • ViSiON/3 BBS Server (%svision3%s)\n", colorCyan, colorReset)
	fmt.Printf("  • Configuration Tool (%svision3-config%s)\n", colorCyan, colorReset)
	fmt.Printf("  • Utility Tools (%sansitest, stringtool%s)\n", colorCyan, colorReset)
	fmt.Printf("  • Menu Sets and ANSI Art\n")
	fmt.Printf("  • Default Configuration Files\n")
	fmt.Printf("  • SSH Host Keys (if ssh-keygen available)\n")
	fmt.Println()
	fmt.Printf("Installation directory: %s%s%s\n", colorYellow, i.installDir, colorReset)
	fmt.Println()
	
	fmt.Printf("%sProceed with installation? [Y/n]:%s ", colorBold, colorReset)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	
	if input != "" && input != "y" && input != "yes" {
		fmt.Printf("%sInstallation cancelled.%s\n", colorRed, colorReset)
		os.Exit(0)
	}
	fmt.Println()
}

func (i *Installer) performInstall() {
	fmt.Printf("%s[INSTALLING]%s\n", colorBlue+colorBold, colorReset)
	
	// Create installation directory
	fmt.Printf("Creating installation directory... ")
	if err := os.MkdirAll(i.installDir, 0755); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%sOK%s\n", colorGreen, colorReset)
	
	// Create subdirectories first
	fmt.Printf("Creating subdirectories... ")
	dirs := []string{"configs", "data/users", "data/files/general", "data/logs", "log", "menus"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(i.installDir, dir), 0755); err != nil {
			fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
			fmt.Printf("Error creating %s: %v\n", dir, err)
			os.Exit(1)
		}
	}
	fmt.Printf("%sOK%s\n", colorGreen, colorReset)
	
	// Extract embedded files (in a real distribution, these would be embedded)
	i.extractFiles()
	
	// Generate SSH host keys
	i.generateSSHKeys()
	
	// Initialize data files
	i.initializeDataFiles()
	
	// Set permissions
	i.setPermissions()
	
	fmt.Printf("%sInstallation completed successfully!%s\n", colorGreen+colorBold, colorReset)
}

func (i *Installer) extractFiles() {
	fmt.Printf("Installing files... ")
	
	// Get the directory where the installer is running from
	installerDir, err := os.Executable()
	if err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error getting installer location: %v\n", err)
		os.Exit(1)
	}
	sourceDir := filepath.Dir(installerDir)
	
	// Copy binaries
	binSourceDir := filepath.Join(sourceDir, "bin")
	binTargetDir := filepath.Join(i.installDir, "bin")
	if err := i.copyDir(binSourceDir, binTargetDir); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error copying binaries: %v\n", err)
		os.Exit(1)
	}
	
	// Copy configs (but don't overwrite existing SSH keys)
	configSourceDir := filepath.Join(sourceDir, "configs")
	configTargetDir := filepath.Join(i.installDir, "configs")
	if err := i.copyDir(configSourceDir, configTargetDir); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error copying configs: %v\n", err)
		os.Exit(1)
	}
	
	// Copy menus
	menuSourceDir := filepath.Join(sourceDir, "menus")
	menuTargetDir := filepath.Join(i.installDir, "menus")
	if err := i.copyDir(menuSourceDir, menuTargetDir); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error copying menus: %v\n", err)
		os.Exit(1)
	}
	
	// Copy data directory (template files, not user data)
	dataSourceDir := filepath.Join(sourceDir, "data")
	dataTargetDir := filepath.Join(i.installDir, "data")
	if err := i.copyDir(dataSourceDir, dataTargetDir); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error copying data: %v\n", err)
		os.Exit(1)
	}
	
	// Copy README and LICENSE
	for _, file := range []string{"README.md", "LICENSE", "ViSiON3.png"} {
		src := filepath.Join(sourceDir, file)
		dst := filepath.Join(i.installDir, file)
		if err := i.copyFile(src, dst); err != nil {
			// These are optional, just warn
			fmt.Printf("\nWarning: Could not copy %s: %v", file, err)
		}
	}
	
	fmt.Printf("%sOK%s\n", colorGreen, colorReset)
}

// copyDir recursively copies a directory
func (i *Installer) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		
		dstPath := filepath.Join(dst, relPath)
		
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		
		return i.copyFile(path, dstPath)
	})
}

// copyFile copies a single file
func (i *Installer) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	
	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	
	return os.Chmod(dst, sourceInfo.Mode())
}

func (i *Installer) generateSSHKeys() {
	fmt.Printf("Generating SSH host keys... ")
	
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		fmt.Printf("%sSKIPPED (ssh-keygen not found)%s\n", colorYellow, colorReset)
		return
	}
	
	configsDir := filepath.Join(i.installDir, "configs")
	
	// Generate RSA key
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-f", filepath.Join(configsDir, "ssh_host_rsa_key"), "-N", "")
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error generating RSA key: %v\n", err)
		return
	}
	
	// Generate Ed25519 key
	cmd = exec.Command("ssh-keygen", "-t", "ed25519", "-f", filepath.Join(configsDir, "ssh_host_ed25519_key"), "-N", "")
	if err := cmd.Run(); err != nil {
		fmt.Printf("%sWARNING%s\n", colorYellow, colorReset)
		fmt.Printf("Error generating Ed25519 key: %v\n", err)
	}
	
	fmt.Printf("%sOK%s\n", colorGreen, colorReset)
}

func (i *Installer) initializeDataFiles() {
	fmt.Printf("Initializing data files... ")
	
	// Create users.json
	usersPath := filepath.Join(i.installDir, "data", "users", "users.json")
	if err := os.WriteFile(usersPath, []byte("{}"), 0644); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error creating users.json: %v\n", err)
		os.Exit(1)
	}
	
	// Create oneliners.json
	onelinersPath := filepath.Join(i.installDir, "data", "oneliners.json")
	if err := os.WriteFile(onelinersPath, []byte("[]"), 0644); err != nil {
		fmt.Printf("%sFAILED%s\n", colorRed, colorReset)
		fmt.Printf("Error creating oneliners.json: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("%sOK%s\n", colorGreen, colorReset)
}

func (i *Installer) setPermissions() {
	fmt.Printf("Setting file permissions... ")
	
	// Set permissions on SSH keys
	configsDir := filepath.Join(i.installDir, "configs")
	
	if files, err := filepath.Glob(filepath.Join(configsDir, "ssh_host_*_key")); err == nil {
		for _, file := range files {
			os.Chmod(file, 0600)
		}
	}
	
	if files, err := filepath.Glob(filepath.Join(configsDir, "ssh_host_*_key.pub")); err == nil {
		for _, file := range files {
			os.Chmod(file, 0644)
		}
	}
	
	fmt.Printf("%sOK%s\n", colorGreen, colorReset)
}

func (i *Installer) showCompletion() {
	fmt.Println()
	fmt.Printf("%s%s", colorGreen+colorBold, strings.Repeat("=", 70))
	fmt.Printf("%s\n", colorReset)
	fmt.Printf("%s               INSTALLATION COMPLETED SUCCESSFULLY!%s\n", colorGreen+colorBold, colorReset)
	fmt.Printf("%s%s", colorGreen+colorBold, strings.Repeat("=", 70))
	fmt.Printf("%s\n", colorReset)
	fmt.Println()
	
	fmt.Printf("%sViSiON/3 BBS has been installed to:%s\n", colorBold, colorReset)
	fmt.Printf("  %s%s%s\n", colorCyan, i.installDir, colorReset)
	fmt.Println()
	
	fmt.Printf("%sNext Steps:%s\n", colorBold, colorReset)
	fmt.Printf("1. Run the configuration tool:\n")
	fmt.Printf("   %s%s/bin/vision3-config%s\n", colorCyan, i.installDir, colorReset)
	fmt.Printf("2. Start the BBS server:\n")
	fmt.Printf("   %s%s/bin/vision3%s\n", colorCyan, i.installDir, colorReset)
	fmt.Printf("3. Connect to your BBS:\n")
	fmt.Printf("   %sssh felonius@localhost -p 2222%s\n", colorCyan, colorReset)
	fmt.Printf("   Default password: %spassword%s\n", colorYellow, colorReset)
	fmt.Println()
	
	fmt.Printf("%sIMPORTANT:%s Change the default password after first login!\n", colorRed+colorBold, colorReset)
	fmt.Println()
	
	fmt.Printf("%sFor file transfer support, install lrzsz:%s\n", colorYellow, colorReset)
	switch i.platform {
	case "darwin":
		fmt.Printf("  %sbrew install lrzsz%s\n", colorCyan, colorReset)
	case "linux":
		fmt.Printf("  %ssudo apt-get install lrzsz%s (Debian/Ubuntu)\n", colorCyan, colorReset)
		fmt.Printf("  %ssudo yum install lrzsz%s (RHEL/CentOS)\n", colorCyan, colorReset)
	case "windows":
		fmt.Printf("  Download lrzsz for Windows or use WSL\n")
	}
	
	fmt.Println()
	fmt.Printf("%sEnjoy your ViSiON/3 BBS!%s\n", colorGreen+colorBold, colorReset)
	fmt.Println()
	
	// Pause before exit
	fmt.Printf("Press ENTER to exit...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}