# git-pr - Git pull request posting tool

Automatically creates pull request adding all the members of your
favorite team (`velocloud/dp`) as reviewers without having to use Web
UI.

By default all the team members will be added as both reviewers and
watchers, during the PR submission the selections could be ajusted.

By convention, the teammates in the reviewer list will be expected to
go through the review and make a verdict (thoughtful comments are
plus).  The watchers - or @ mentions - will be notified but their
input is not expected, although appreciated.

So, dring the PR submission, the autor will reduce the list of
reviewers (by deleting `Review-By` lines) but

## Install

This tool is written in Go - to build it you need a reasonably recent
Go toolchain. `ap-get install golang` will likely suffice.

In velocloud workspace - <vcroot>

```
export GOPATH=<vcroot>/dev/vadim/
cd $GOPATH/src/git-pr
go get ./...

```

If everything is fine this will result in a binary
`<vcroot>/dev/vadim/bin/git-pr`, the resulting executable is self
contained and usable on pretty much any x86_64 Linux.

`<vcroot>/dev/vadim/bin/git-pr install` will create a git command alias
that will allow to invoke this program as `git pr`.


It is best to create an `app password` in your git account settings
giving it the limited set of priviledges (Not sure exactly - read
access to teams and write to pull request as a minimum)

It will create an ugly token that you can place in ~/.bitbucket
together with your user ID and your favorite team.

```
{
    "user" : "john_doe",
    "password" : "*********",
    "team" : "vcdp"
}
```

In case post-commit hook `dev/vadim/post-commit` is used it is
necessary to configure the remote branch naming conversion.  Add
`"branch" : "{{args.User}}/{{args.Branch}}"` to ~/.bitbucket json.


## Standard pull request

If set up as described below, `git pr` will create a PR draft and vill
launch your default editor to allow to make changes if neccessary.

The assumption is that the PR is created from the current branch in
your workspace is created to follow `upstream` (`git --set-upstream-to=<upstream>`), 
that this branch `branch` is already pushed into the remote,
and that the PR is created to merge the `branch` into the `upstream`.

These assumptions are satisfied if you created your topic branch off
the target upstream branch. And then after committing your changes you
pushed them upstream `git push`

```
git checkout origin/master -b my-fix
git commit
git push
git pr
```


They are also satisfied if you are using post-commit hook
(`dev/vadim/post-commit`) that backs up the changes in your local branch
by force-pushing them into a private upstream branch. In this case if
you are working on `git checkout <branch>` and some changes have been
committed locally, they are already pushed remotely as `<user>/<branch>`.

e.g.
```
git checkout release_3.2
git commit ...
git pr
```

The PR draft consists of the PR title and description - it will be
pre-poulated with the all commit description that your local branch
has on top of the upstream - and a list of designated reviewers and
team members to notify prepopulated with your team members.


```
#
# All lines starting with # will be removed. Of the remaining the first
# line will be used as a title and the rest as description.
#
Short and descriptove subject

Long and helpful description.

# This PR will be sent to the following recipients:
Notify: @craigconnors @gopavelo @jordanrhody @kartik_vc
Review-By: craigconnors <Craig Connors>
Review-By: kartik_vc <Kartik Kamdar>
```

You can modify the generated description to your liking, save and exit
editor.  All the comment lines (starting with `#`) will be removed,
the first line will be used as a PR title and thhe rest will
constitute the PR description.

Except all the lines starting with `Review-By: ` will be coverted to
the list of reviewers.  The `Notify:` line will remain n the PR
description and the 'mention' email will be sent to those recipients
(or any @ mention anywhere in the description).





