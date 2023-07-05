---
title: "Setup Docker"
draft: false
weight: 20
---

## Software

Install Docker on all servers/vms: https://docs.docker.com/engine/install/

## Create Swarm

On one of the servers, initialize the swarm:

```bash
$ docker swarm init

> Swarm initialized: current node (bvz81updecsj6wjz393c09vti) is now a manager.
> 
> To add a worker to this swarm, run the following command:
> 
>     docker swarm join \
>     --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-1awxwuwd3z9j1z3puu7rcgdbx \
>     172.17.0.2:2377
> 
> To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.
```

On the other servers, join the swarm:

```bash
$ docker swarm join \
    --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-1awxwuwd3z9j1z3puu7rcgdbx \
    172.17.0.2:2377
```

Read the administration guide for docker swarm carefully and make the appropriate adjustments for your environment:
https://docs.docker.com/engine/swarm/admin_guide/

Verify that all nodes are in the swarm:

```bash
$ docker node ls
ID                                HOSTNAME         STATUS    AVAILABILITY   MANAGER STATUS   ENGINE VERSION
ko66mqj76xo9ftunxq78luc8p         vm01             Ready     Active         Reachable        23.0.1
ogziph0qxfeivly5fnekepwx0         vm02             Ready     Active                          23.0.1
1prl8b1m7xw2ph5b8dnh98glk         vm03             Ready     Active                          23.0.1
8utl07361ocn5xvzqh27z0c8s *       vm04             Ready     Active         Reachable        23.0.1
hdsuhlwecidor7khbcfn4gni3         vm05             Ready     Active         Reachable        23.0.1
hj6fkl3j5hwho40uiehc7ikq5         vm06             Ready     Active         Leader           23.0.1
ctfdd9mtkse2yxid8zku2wx1f         vm07             Ready     Active                          23.0.1
u391iukj6nljosaaygcfkzy2s         vm08             Ready     Active                          23.0.1
wkxct5tvzclvc4uqm8w573dlf         vm09             Ready     Active                          23.0.1
72weo6nozra1cdgjs5wghe7gh         vm10             Ready     Active                          23.0.1
```

## Tag nodes

We use labels to tag our nodes and to deploy services to appropriate nodes.

This commands adds the label worker to the node vm02, instructing our deployment to deploy workers on this node:

```bash
docker node update --label-add worker=true vm02
```
This is a configuration you should aim for:

```bash
docker node ls -q | xargs docker node inspect -f '{{ .ID }} [{{ .Description.Hostname }}]: {{ .Spec.Labels }}'

kwgmm6sxb9nqwojoclxuy4mpt [vmgpu01]: map[voiceservice:true] # optional, this is a server with a GPU for transcription
ko66mqj76xo9ftunxq78luc8p [vm01]: map[db:true traefik:true tumlive:true] # this server is important, it runs the database and the reverse proxy. Don't under-provision it.
hj6fkl3j5hwho40uiehc7ikq5 [vm02]: map[grafana:true influx:true meilisearch:true monitoring:true prometheus:true] # these services are not critical - and optional
ctfdd9mtkse2yxid8zku2wx1f [vm03]: map[worker:true] # the number of workers depends on the number of concurrent streams you want to process. 1 worker can process around 5 stream in our environment.
u391iukj6nljosaaygcfkzy2s [vm04]: map[worker:true]
wkxct5tvzclvc4uqm8w573dlf [vm05]: map[worker:true]
72weo6nozra1cdgjs5wghe7gh [vm06]: map[worker:true]
f7ik66qq6tzhsbwphfpdp2vm1 [vm07]: map[worker:true]
i4l8ouumms96qu96evkb6srol [vm08]: map[worker:true]
vq5cw2bgwncenr5cp89xzsi32 [vm09]: map[worker:true]
q4as4i27z2hnwypgzj8ql2dz1 [vm10]: map[worker:true]
lfged5ra1a7z9wlstxa2bml5c [vm11]: map[worker:true]
3wu812ybzynnunrpoqdsay0bf [vm12]: map[worker:true]
itdbo77gempnl251lakioe5y1 [vm13]: map[worker:true]
zcplsihexr88plf0t8q25tdn7 [vm14]: map[worker:true]
fbi92hp7s0u3c2x13tgrb6fd6 [vm15]: map[worker:true]
o6k2egpupik3qjgq2w0azv70o [vm16]: map[worker:true]
urac70xjf1kx5op39kyulykad [vm17]: map[worker:true]
wpue8f384h7z71mngov5j72c1 [vm18]: map[worker:true]
th77fn3s91s06sy4ciprita3s [vm19]: map[edge:true] # the number of edge nodes depends on the number of concurrent viewers you want to support.
5bqr01nyefxqmkd3luzhh3sne [vm20]: map[edge:true]
vrroo1k8kgk8n557pos5wlz5k [vm21]: map[edge:true]
b6m40kbtg1sctwq5p4vmtghxd [vm22]: map[edge:true]

```
