package flexbot

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/config"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ucsm"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/util/crypt"
	"github.com/igor-feoktistov/terraform-provider-flexbot/pkg/ontap"
)

// Default timeouts
const (
	NodeRestartTimeout = 600
	NodeGraceShutdownTimeout = 120
	NodeGracePowerOffTimeout = 10
	StorageRetryAttempts = 5
	StorageRetryTimeout = 15
)

// Change status definition while update routine
const (
	ChangeBladeSpec       = 1
	ChangePowerState      = 2
	ChangeSnapshotCreate  = 4
	ChangeSnapshotDelete  = 8
	ChangeSnapshotRestore = 16
	ChangeOsImage         = 32
	ChangeSeedTemplate    = 64
	ChangeBootDiskSize    = 128
	ChangeDataDiskSize    = 256
	ChangeDataDisk        = 512
)

func checkSSHListen(host string) (listen bool) {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", host+":22", timeout)
	if err != nil {
		listen = false
	} else {
		listen = true
		conn.Close()
	}
	return
}

func checkSSHCommand(host string, sshUser string, sshPrivateKey string) (err error) {
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		return
	}
	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: time.Second * time.Duration(15),
	}
	if conn, err = ssh.Dial("tcp", host+":22", config); err != nil {
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		return
	}
	err = sess.Run("uname -a")
	sess.Close()
	return
}

func waitForSSH(nodeConfig *config.NodeConfig, waitForSSHTimeout int, sshUser string, sshPrivateKey string) (err error) {
	giveupTime := time.Now().Add(time.Second * time.Duration(waitForSSHTimeout))
	restartTime := time.Now().Add(time.Second * NodeRestartTimeout)
	for time.Now().Before(giveupTime) {
		if checkSSHListen(nodeConfig.Network.Node[0].Ip) {
			if len(sshUser) > 0 && len(sshPrivateKey) > 0 {
				stabilazeTime := time.Now().Add(time.Second * 60)
				for time.Now().Before(stabilazeTime) {
					if err = checkSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey); err == nil {
						break
					}
					time.Sleep(5 * time.Second)
				}
			}
			if err == nil {
				break
			}
		}
		time.Sleep(5 * time.Second)
		if time.Now().After(restartTime) {
			ucsm.StopServer(nodeConfig)
			ucsm.StartServer(nodeConfig)
			restartTime = time.Now().Add(time.Second * NodeRestartTimeout)
		}
	}
	if time.Now().After(giveupTime) {
		if  err == nil {
			err = fmt.Errorf("waitForSsh(): exceeded timeout %d", waitForSSHTimeout)
		} else {
			err = fmt.Errorf("waitForSsh(): exceeded timeout %d: %s", waitForSSHTimeout, err)
		}
	} else {
		if  err != nil {
			err = fmt.Errorf("waitForSsh(): %s", err)
		}
	}
	return
}

func runSSHCommand(sshHost string, sshUser string, sshPrivateKey string, command string) (commandOutput string, err error) {
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var bStdout, bStderr bytes.Buffer
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("runSSHCommand(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: time.Second * time.Duration(15),
	}
	if conn, err = ssh.Dial("tcp", sshHost+":22", config); err != nil {
		err = fmt.Errorf("runSSHCommand(): failed to connect to host %s: %s", sshHost, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("runSSHCommand(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	sess.Stdout = &bStdout
	sess.Stderr = &bStderr
	err = sess.Run(command)
	if err != nil {
		err = fmt.Errorf("runSSHCommand(): failed to run command %s: %s: %s", command, err, bStderr.String())
		return
	}
	if bStdout.Len() > 0 {
		commandOutput = bStdout.String()
	}
	return
}

func shutdownServer(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string) (err error) {
        var operState string
        // Trying graceful node shutdown
	if _, err = runSSHCommand(nodeConfig.Network.Node[0].Ip, sshUser, sshPrivateKey, "sudo shutdown -h 0"); err == nil {
	        waitForShutdown := time.Now().Add(time.Second * time.Duration(NodeGraceShutdownTimeout))
	        for time.Now().Before(waitForShutdown) {
	        	if operState, err = ucsm.GetServerOperationalState(nodeConfig); err != nil {
			        return
		        }
		        if operState == "power-off" {
		                break
		        }
		        time.Sleep(5 * time.Second)
	        }
	}
	err = ucsm.StopServer(nodeConfig)
        return
}

func waitForOperationalState(nodeConfig *config.NodeConfig, state string, waitTimeout int) (err error) {
	var currentState string
	giveupTime := time.Now().Add(time.Second * time.Duration(waitTimeout))
	for time.Now().Before(giveupTime) {
		if currentState, err = ucsm.GetServerOperationalState(nodeConfig); err != nil {
			return
		}
		if currentState == state {
			return
		}
		time.Sleep(5 * time.Second)
	}
	err = fmt.Errorf("waitForOperationalState(): exceeded timeout %d, state=%s", waitTimeout, currentState)
	return
}

func waitForPowerState(nodeConfig *config.NodeConfig, state string, waitTimeout int) (err error) {
	var currentState string
	giveupTime := time.Now().Add(time.Second * time.Duration(waitTimeout))
	for time.Now().Before(giveupTime) {
		if currentState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
			return
		}
		if currentState == state {
			return
		}
		time.Sleep(5 * time.Second)
	}
	err = fmt.Errorf("waitForPowerState(): exceeded timeout %d, state=%s", waitTimeout, currentState)
	return
}

func waitForHostNetwork(nodeConfig *config.NodeConfig, waitTimeout int) (err error) {
	giveupTime := time.Now().Add(time.Second * time.Duration(waitTimeout))
	for time.Now().Before(giveupTime) {
		if checkSSHListen(nodeConfig.Network.Node[0].Ip) {
			return
		}
		time.Sleep(5 * time.Second)
	}
	err = fmt.Errorf("waitForHostNetwork(): exceeded timeout %d", waitTimeout)
	return
}

func decryptAttribute(meta interface{}, encrypted string) (decrypted string, err error) {
	meta.(*config.FlexbotConfig).Sync.Lock()
	defer meta.(*config.FlexbotConfig).Sync.Unlock()
	if decrypted, err = crypt.DecryptString(encrypted, meta.(*config.FlexbotConfig).FlexbotProvider.Get("pass_phrase").(string)); err != nil {
		err = fmt.Errorf("decryptAttribute(): failure to decrypt: %s", err)
	}
	return
}

func createSnapshot(nodeConfig *config.NodeConfig, sshUser string, sshPrivateKey string, snapshotName string) (err error) {
	var filesystems, freezeCmds, unfreezeCmds, errs []string
	var signer ssh.Signer
	var conn *ssh.Client
	var sess *ssh.Session
	var bStdout, bStderr bytes.Buffer
	var exists bool
	if exists, err = ontap.SnapshotExists(nodeConfig, snapshotName); exists || err != nil {
		return
	}
	if signer, err = ssh.ParsePrivateKey([]byte(sshPrivateKey)); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to parse SSH private key: %s", err)
		return
	}
	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if conn, err = ssh.Dial("tcp", nodeConfig.Network.Node[0].Ip+":22", config); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to connect to host %s: %s", nodeConfig.Network.Node[0].Ip, err)
		return
	}
	defer conn.Close()
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	sess.Stdout = &bStdout
	sess.Stderr = &bStderr
	err = sess.Run(`cat /proc/mounts | sed -n 's/^\/dev\/mapper\/[^ ]\+[ ]\+\(\/[^ \/]\{1,64\}\).*/\1/p' | uniq`)
	sess.Close()
	if err != nil {
		err = fmt.Errorf("createSnapshot(): failed to run command: %s: %s", err, bStderr.String())
		return
	}
	if bStdout.Len() > 0 {
		filesystems = strings.Split(strings.Trim(bStdout.String(), "\n"), "\n")
	}
	unfreezeCmds = append(unfreezeCmds, "fsfreeze -u /")
	for _, fs := range filesystems {
		freezeCmds = append(freezeCmds, "fsfreeze -f "+fs)
		unfreezeCmds = append(unfreezeCmds, "fsfreeze -u "+fs)
	}
	freezeCmds = append(freezeCmds, "fsfreeze -f /")
	cmd := fmt.Sprintf(`sudo -n sh -c 'sync && sleep 5 && sync && %s && (echo -n frozen && sleep 5); %s'`, strings.Join(freezeCmds, " && "), strings.Join(unfreezeCmds, "; "))
	if sess, err = conn.NewSession(); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to create SSH session: %s", err)
		return
	}
	defer sess.Close()
	bStdout.Reset()
	bStderr.Reset()
	sess.Stdout = &bStdout
	sess.Stderr = &bStderr
	if err = sess.Start(cmd); err != nil {
		err = fmt.Errorf("createSnapshot(): failed to start SSH command: %s", err)
		return
	}
	for i := 0; i < 30; i++ {
		if bStdout.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if bStdout.String() == "frozen" {
		if err = ontap.CreateSnapshot(nodeConfig, snapshotName, ""); err != nil {
			errs = append(errs, err.Error())
		}
	} else {
		errs = append(errs, "fsfreeze did not complete, snapshot is not created")
	}
	if err = sess.Wait(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to run SSH command: %s: %s", err, bStderr.String()))
	}
	if err != nil {
		err = fmt.Errorf("createSnapshot(): %s", strings.Join(errs, " , "))
	}
	return
}

func stringSliceIntersection(src1, src2 []string) (dst []string) {
	hash := make(map[string]bool)
	for _, e := range src1 {
		hash[e] = true
	}
	for _, e := range src2 {
		if hash[e] {
			dst = append(dst, e)
		}
	}
	return
}

func stringSliceElementExists(array []string, elem string) bool {
	for _, e := range array {
		if e == elem {
			return true
		}
	}
	return false
}

func valueInRange(rangeStr string, value int) (bool, error) {
	var err error
	var rangeLower, rangeUpper int
	re := regexp.MustCompile(`([0-9]+)\s*-\s*([0-9]*)`)
	subMatch := re.FindStringSubmatch(rangeStr)
	if len(subMatch) == 3 {
		if rangeLower, err = strconv.Atoi(subMatch[1]); err != nil {
			return false, err
		}
		if rangeUpper, err = strconv.Atoi(subMatch[2]); err != nil {
			return false, err
		}
		return (value >= rangeLower && value <= rangeUpper), nil
	}
	if rangeLower, err = strconv.Atoi(rangeStr); err != nil {
		return false, err
	}
	return (rangeLower == value), nil
}
