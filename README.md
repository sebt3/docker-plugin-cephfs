# docker-plugin-cephfs

A very simple/naive docker swarm plugin for cephfs

## Why yet an other cephfs docker plugin ?
As far as I know, all the others are either completly outdated, or works only as local plugin (doesnt work in a swarm environement)
If you know any, please let me know.

## Requierments
Each swarm nodes should have a correctly populated /etc/ceph directory

## Install

On each docker nodes :
```sh
docker plugin install --alias cephfs sebt3/docker-plugin-cephfs:master --grant-all-permissions
```

## Configure
```sh
docker plugin disable cephfs --force
ceph auth get-or-create client.dockeruser mon 'allow r' osd 'allow rw' mds 'allow' > /etc/ceph/ceph.client.dockeruser.keyring
docker plugin set cephfs CLIENT_NAME=dockeruser
docker plugin enable cephfs
```

## Turn debuging on
Warning, dont do this on production, as it's going to leak your authentification keys in the logs
```sh
docker plugin disable cephfs --force
docker plugin set cephfs DEBUG=1
docker plugin enable cephfs
```
Check the logs 
```sh
journalctl -xeu docker|grep CEPHFS
```

## How does it works
It doesnt use any databases. All it does is creating a /docker directory in your cephfs. Every subdirectories are seen as a volume. This way all the nodes see the same volumes easyly
