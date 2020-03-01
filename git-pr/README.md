# git-pr - Git pull request posting tool

Automatically creates pull request adding the members of your favorite
team (`velocloud/dp`) as reviewers without having to use Web UI.  With
right label and stuff. It can also launch jenkis regression job on
your branch.

The behaviour is controlled by configuration/command-line options and
'trailers' - `Review-By: xxx` or `Jenkins-Suite: vpn` markup at the
end of the MR description.


## Install

This tool is written in Go - to build it you need a reasonably recent
Go toolchain. The best way to get it is to obtain the latest master
OpenWRT sdk with gettoolchain.py

In velocloud workspace - <vcroot>

```
mkdir -p dev/vadim/src
git clone git@gitlab.eng.vmware.com:egorovv/gotools dev/vadim/src/gotools
make -C dev/vadim/src/gotools [SDK=<sdk> if not set in local.mk]

```

If everything is fine this will result in a binary
`<vcroot>/dev/vadim/bin/git-pr`, the resulting executable is self
contained and usable on pretty much any x86_64 Linux.

You need to create an `access token` in your git account settings -
`setting/access tokens` - with api permissins.

```
<vcroot>/dev/vadim/bin/git-pr install --team velocloud/dp --owner velocloud \
    --label engineering_dataplane --user <userid> --password <token>
```

This will create a git command alias that will allow to invoke this
program as `git pr`.
The settings provided will be saved as defaults in  your `~/.gitconfig`.

## Jenkins integration

After MR submission a jenkins job (by default
`devtest-pvt-branch-validator`) can be launched to run regression
tests. A comment will be added to MR to record the link to the job.

To enable jenkins integration - an api token needs to be set up in
your jenkins account - `your account - Configure - API token - Add new
token`.

The token can be supplied during install (or invocation) using
`--jenkins-token` 

`--jenkins-suite` option control the testsuite that will be run -
'bronze' is the default, it can be adjusted during MR creation via
`Jenkins-Suite:` trailer




## Standard pull request

```
git checkout origin/master -b my-fix
git commit
git pr
```
or cherry pick
```
git checkout origin/release_3.2 -b my-backport
git cherry-pick <xyz>
git pr
```

If set up as described above, `git pr` will create a PR draft and will
launch your default editor to allow making changes if neccessary.

The current branch will be pushed into the remote, and merge request
will be made from the remote btanch to upstream branch (`git
--set-upstream-to=<upstream>`).

The PR draft consists of the PR title and description - it will be
pre-poulated with the all commit description that your local branch
has on top of the upstream - and a list of designated reviewers and
team members to notify prepopulated with your team members.


```
#
# All lines starting with # will be removed. Of the remaining the first
# line will be used as a title and the rest as description.
#
Short and descriptive subject

Long and helpful description.

# This PR will be sent to the following recipients:
Review-By: craigconnors <Craig Connors>
Review-By: kartik_vc <Kartik Kamdar>
```

You can modify the generated description to your liking, save and exit
editor.  All the comment lines (starting with `#`) will be removed,
the first line will be used as a PR title, Except all the lines
starting with `Review-by: ` will be coverted to the list of reviewers
and and the rest will constitute the PR description.





