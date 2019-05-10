# veth-netns

Given a veth network interface name, this tool can print the network namespace
inode number of the network namespace where the other end of the veth pair
lives.

## Example

We'll start a container and find out the ifindex of its network interface and
the inode number of its network namespace:

```
$ sudo rkt run --interactive kinvolk.io/aci/busybox
/ # ip link
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
3: eth0@if23: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue 
    link/ether fe:5d:98:92:a1:9e brd ff:ff:ff:ff:ff:ff
/ # readlink /proc/self/ns/net
net:[4026532431]

Now, on the host, we check the name of the network interface that's paired with
the container's.

...

$ ip link | grep 23
23: vethb7b583ef@if3: <BROADCAST,MULTICAST,DYNAMIC,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
```

veth-netns can take the network interface name on the host and figure out the
network namespace inode number of the container:

```
$ sudo ./veth-netns vethb7b583ef
link-netnsid of vethb7b583ef: 0
netns ino ID for vethb7b583ef: 4026532431
```
