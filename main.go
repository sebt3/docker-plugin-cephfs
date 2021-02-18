/*
 * Copyright 2020 Sébastien Huss <sebastien.huss@gmail.com>
 * 
 * Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
 * 
 * 1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
 * 
 * 2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
 * 
 * 3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.
 * 
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main

import (
	"fmt"
	"log"
	"sync"
	"path"
	"syscall"
	"os"

	"gopkg.in/ini.v1"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/cephfs"
)

const (
	socket	= "cephfs"
	baseDir	= "/docker"
)

type Volume struct {
	name		string
	ID		string
	mountPath	string
	mounts		int64
}

type Driver struct {
	debug		bool
	username	string
	secret		string
	servers		string
	volumes		map[string]*Volume
	sync.RWMutex
}
func newDriver(user, sec, svrs string, debug bool) (*Driver, error) {
	ret := &Driver {
		debug:		debug,
		username:	user,
		secret:		sec,
		servers:	svrs,
		volumes:	map[string]*Volume{},
	}
	return ret, ret.connect()
}
func (d *Driver) add(name, ID, mountPath string, mounts int64) error {
	var err error
	if _, found := d.volumes[name]; found {
		d.volumes[name].mounts++
		if d.volumes[name].ID == "" {
			d.volumes[name].ID = ID
		} else if ID != "" && d.volumes[name].ID != ID {
			err =  fmt.Errorf("volume ID don't match: stored(%s), given(%s)",d.volumes[name].ID, ID)
		}
		if d.volumes[name].mountPath == "" {
			d.volumes[name].mountPath = mountPath
		} else if mountPath != "" && d.volumes[name].mountPath != mountPath {
			err =  fmt.Errorf("volume mountPath don't match: stored(%s), given(%s)",d.volumes[name].mountPath, mountPath)
		}
	} else {
		d.volumes[name] = &Volume {
			name:		name,
			ID:		ID,
			mountPath:	mountPath,
			mounts:		mounts,
		}
	}
	return err
}

func getEnv(name, defaut string) string {
	val, found := os.LookupEnv(name)
	if found {
		return val
	}
	return defaut
}

/*
 * FileSystem Management
 */
func (d *Driver) cephfsMount(mountPoint, cephPath string) error {
	if d.debug {
		log.Printf("CEPHFS: cephfsMount: %s:%s %s ceph 0 name=%s,secret=%s\n",d.servers,cephPath, mountPoint, d.username,d.secret)
	}
	return syscall.Mount(fmt.Sprintf("%s:%s",d.servers,cephPath), mountPoint, "ceph", 0, fmt.Sprintf("name=%s,secret=%s",d.username,d.secret))
}
func (d *Driver) cephfsUMount(mountPoint string) error {
	return syscall.Unmount(mountPoint, 0)
}

/*
 * Ceph Management
 */
func (d *Driver) connect() error {
	if d.debug {
		log.Printf("CEPHFS: INFO: Connecting to ceph\n")
	}
	var err error
	cephConn,err := rados.NewConn()
	if err != nil {
		log.Panicf("CEPHFS: ERROR: Unable to create ceph connection")
	}
	err = cephConn.ReadDefaultConfigFile()
	if err != nil {
		log.Panicf("CEPHFS: ERROR: Unable to read ceph default configuration")
	}
	err = cephConn.Connect()
	if err != nil {
		log.Panicf("CEPHFS: ERROR: Unable to connect to ceph: %s",err)
	}
	return nil
}

func (d *Driver) createMountInfo() (*cephfs.MountInfo, error) {
	info, err := cephfs.CreateMount()
	if err!= nil {
		if d.debug {
			log.Printf("CEPHFS: ERROR: cephfs.CreateMount: %s\n",err)
		}
		return nil,err
	}
	err = info.ReadDefaultConfigFile()
	if err != nil {
		if d.debug {
			log.Printf("CEPHFS: ERROR: info.ReadDefaultConfigFile: %s\n",err)
		}
		return nil,err
	}
	info.Mount()
	info.ChangeDir(baseDir)
	dir := info.CurrentDir()
	if dir != baseDir {
		err = info.MakeDir(baseDir,0755)
		if err != nil {
			if d.debug {
				log.Printf("CEPHFS: ERROR: Cannot create base directory : %s : %s\n",baseDir, err)
			}
			return nil,err
		}
		info.ChangeDir(baseDir)
		dir = info.CurrentDir()
		if dir != baseDir {
			if d.debug {
				log.Printf("CEPHFS: ERROR: Cannot create base directory: %s\n",baseDir)
			}
			return nil, fmt.Errorf("Cannot create base directory: %s\n", baseDir)
		}
	}

	return info,nil
}

/*
 * Plugin interface
 * see : https://github.com/containers/docker-lvm-plugin/blob/master/driver.go
 *       https://github.com/docker/go-plugins-helpers/blob/master/volume/api.go
 */
func (d *Driver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse {
		Capabilities: volume.Capability{Scope: "global"},
	}
}
func (d *Driver) Create(r *volume.CreateRequest) error {
	d.Lock()
	defer d.Unlock()

	if d.debug {
		log.Printf("CEPHFS: INFO: Create Called: %s\n", r.Name)
	}
	info, err := d.createMountInfo()
	if err != nil {
		return err
	}
	if err := d.add(r.Name, "", "", 0);	err != nil {
		return err
	}
	return info.MakeDir(r.Name,0755)
}
func (d *Driver) List() (*volume.ListResponse, error) {
	d.RLock()
	defer d.RUnlock()

	if d.debug {
		log.Printf("CEPHFS: INFO: List Called\n")
	}
	var ls []*volume.Volume
	info, err := d.createMountInfo()
	if err != nil {
		return nil, err
	}
	dir, err := info.OpenDir(baseDir)
	if err != nil {
		return nil,err
	}
	f, err := dir.ReadDir()
	for err == nil && f != nil {
		name := f.Name()
		if name != "." && name != ".." {
			v := &volume.Volume{
				Name: name,
			}
			if _, found := d.volumes[name]; found {
				v.Mountpoint = d.volumes[name].mountPath
			}
			ls = append(ls, v)
		}
		f, err = dir.ReadDir()
	}
	dir.Close()
	return &volume.ListResponse{Volumes: ls}, nil
}
func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	if d.debug {
		log.Printf("CEPHFS: INFO: Get Called: %s\n", r.Name)
	}
	info, err := d.createMountInfo()
	if err != nil {
		return nil,err
	}
	info.ChangeDir(r.Name)
	if info.CurrentDir() == baseDir {
		return nil, fmt.Errorf("could not find %s volume", r.Name)
	}
	vol := &volume.Volume{
		Name:r.Name,
		Mountpoint: path.Join(volume.DefaultDockerRootDirectory, r.Name),
	}
	return &volume.GetResponse{ Volume: vol }, nil
}
func (d *Driver) Remove(r *volume.RemoveRequest) error {
	d.Lock()
	defer d.Unlock()

	if d.debug {
		log.Printf("CEPHFS: INFO: Remove Called: %s\n", r.Name)
	}
	// TODO: Détecter si le volume est encore mounté
	info, err := d.createMountInfo()
	if err != nil {
		return err
	}
	err = info.RemoveDir(r.Name)
	if err != nil {
		// TODO: faire un rm -rf
		return err
	}
	if _, found := d.volumes[r.Name]; found {
		delete(d.volumes, r.Name)
	}
	return nil
}
func (d *Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	d.Lock()
	defer d.Unlock()

	if d.debug {
		log.Printf("CEPHFS: INFO: Mount Called %s, %s\n", r.Name, r.ID)
	}
	// TODO: Check that the mount point is not already mounted
	mp := path.Join(volume.DefaultDockerRootDirectory, r.Name)
	if err := d.add(r.Name,r.ID, mp, 1);	err != nil {
		if d.debug {
			log.Printf("CEPHFS: MOUNT: %s n'était pas connu comme volume: %s\n", r.Name, err)
		}
		mp = d.volumes[r.Name].mountPath
	}
	err := syscall.Mkdir(mp, 0755);
	if err != nil && os.IsNotExist(err) {
		if d.debug {
			log.Printf("CEPHFS: MOUNT: %s création du mointpoint step1: %s\n", r.Name, err)
		}
		err = syscall.Mkdir(volume.DefaultDockerRootDirectory, 0755);
		if err != nil {
			if d.debug {
				log.Printf("CEPHFS: FAILED: MOUNT: %s création du mointpoint step2: %s\n", r.Name, err)
			}
			return nil,err
		}
		err = syscall.Mkdir(mp, 0755);
		if err != nil {
			if d.debug {
				log.Printf("CEPHFS: FAILED: MOUNT: %s création du mointpoint step3: %s\n", r.Name, err)
			}
			return nil,err
		}
	}
	if err != nil && ! os.IsExist(err) {
		if d.debug {
			log.Printf("CEPHFS: FAILED: MOUNT: %s création du mointpoint step4: %s\n", r.Name, err)
		}
		return nil,err
	}
	if d.volumes[r.Name].mounts <= 1 {
		err = d.cephfsMount(mp,path.Join(baseDir, r.Name))
		if err != nil {
			if d.debug {
				log.Printf("CEPHFS: FAILED: MOUNT: %s mount: %s\n", r.Name, err)
			}
			return nil,err
		}
	}
	return &volume.MountResponse{
		Mountpoint: mp,
	}, nil
}
func (d *Driver) Unmount(r *volume.UnmountRequest) error {
	d.Lock()
	defer d.Unlock()

	if d.debug {
		log.Printf("CEPHFS: INFO: Unmount Called %s,%s\n",r.Name,r.ID)
	}
	//TODO: improve error management
	if _, found := d.volumes[r.Name]; found {
		d.volumes[r.Name].mounts--
	}
	if d.volumes[r.Name].mounts == 0 {
		return d.cephfsUMount(path.Join(volume.DefaultDockerRootDirectory, r.Name))
        } else {
		return nil
	}
}
func (d *Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	if d.debug {
		log.Printf("CEPHFS: INFO: Path Called %s\n",r.Name)
	}
	return &volume.PathResponse{Mountpoint: path.Join(volume.DefaultDockerRootDirectory, r.Name)}, nil
}


/*
 * Main stuff
 */
func main() {
	debug:= getEnv("DEBUG", "0")
	user := getEnv("CLIENT_NAME", "admin")
	secr := getEnv("SECRET", "")
	if secr == "" {
		ring, err := ini.Load(fmt.Sprintf("/etc/ceph/ceph.client.%s.keyring",user))
		if err != nil {
			log.Panicf("ERROR: Unable to find the secret key, set CLIENT_NAME and/or SECRET env")
		}
		secr = ring.Section(fmt.Sprintf("client.%s",user)).Key("key").String()
	}
	serv := getEnv("SERVERS", "")
	if serv == "" {
		cfg, err := ini.Load("/etc/ceph/ceph.conf")
		if err == nil {
			serv = cfg.Section("global").Key("mon host").String()
		} else {
			serv = "192.168.1.1"
		}
	}

	d, err := newDriver(user, secr, serv, debug=="1")
	if err != nil {
		log.Panicf("ERROR: %s", err)
	}
	hand := volume.NewHandler(d)
	log.Panic(hand.ServeUnix(socket, 1))
}
