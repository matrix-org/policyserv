# Contributing to policyserv

Welcome! This guide should help you get started contributing to policyserv. If you haven't already joined 
[#policyserv:matrix.org](https://matrix.to/#/#policyserv:matrix.org) on Matrix, we recommend that you do to get the best
possible support.

If you're looking to suggest features or report bugs, please use the [issue tracker](https://github.com/matrix-org/policyserv/issues).
Issues tagged with `good first issue` are a good place to start for developers, and `help wanted` are things we could use 
help with.

Note that this project is relatively complicated compared to other Matrix projects, especially in the Trust & Safety space.
We assume that you have an understanding of the [Matrix specification](https://spec.matrix.org) and either experience or
determination to explore the codebase and Federation API yourself. If you run into issues, we'll do our best to help you
out in [our Matrix room](https://matrix.to/#/#policyserv:matrix.org), but do encourage digging through the available
specification and documentation first.

## Who can contribute

Everyone is welcome to contribute code to Matrix (https://github.com/matrix-org), provided that they are willing to 
license their contributions under the same license as the project itself. We follow a simple 'inbound=outbound' model for 
contributions: the act of submitting an 'inbound' contribution means that the contributor agrees to license the code 
under the same terms as the project's overall 'outbound' license - in our case, this is [Apache Software License v2](./LICENSE).

## Development

Policyserv is written in Go, and runs as a minimal Matrix homeserver implementation. It can be a little complicated to
get running locally, but it's not impossible (some instructions are provided later on). For this reason, policyserv heavily 
relies on unit and integration tests to ensure that it works as expected.

After forking and cloning the repository, run `go test ./... -parallel 10` to ensure the copy of the code you've got is
starting at a known-good state. If the tests fail, visit the [#policyserv:matrix.org](https://matrix.to/#/#policyserv:matrix.org)
room on Matrix to ask for help.

When you're ready to make changes, create a feature branch, write the code, and submit a pull request to our `main` branch.
Please ensure new code is covered by tests where possible, and that the tests pass. Code style is enforced by our linters
and can be checked locally with the following:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
go vet ./cmd/...
staticcheck ./cmd/...
```

## Architectural principles

Policyserv is written to:

* Be fast and efficient on the hot path
* Be easy to extend with new filters
* Scale horizontally by assuming multiple instances will be running at any one time
* Support communities on Matrix, whether they're single rooms, spaces, or entire servers 
* Be highly testable through dependency injection principles
* Be easy to diagnose and debug at 3am after a page (this is more of an aspirational goal at the moment)
* Not be overly specific in its filter support (ie: we don't really want a filter that hardcodes what is allowable content)
* Think in terms of "spammy" and "neutral" rather than "spam" and "not spam"

## Definitions / Architecture

**Filter**: Code which analyzes an event in isolation from other filters, producing a series of classifications and a
"neutral" or "spammy" determination.

**Set Group**: The precise filters to run on an event for a given stage. All filters in a set group are executed
concurrently. Can be used to "pre" and "post" process events. For example, within a filter set, the first set group may
check to see if the event's sender is an admin to prevent later set groups from taking action against them. The last set
group may inspect the result from the prior set groups to have future events be flagged as spammy right away.

**(Filter) Set**: Made up of set groups. Carries configuration information and code dependencies specific to a community
or room into the filters that community would like to run against events. Internally, the set groups which make up the
filter set are ordered to provide some priority filtering and efficient processing.

**Confidence Vector**: A float value between 0 and 1 to carry confidence information about a classification. Zero is low/no
confidence that the classification applies while 1 is high/total confidence.

**Classification**: A measurable and useful datum about an event. The most common being "spam". Events may have multiple
classifications through confidence vectors.

## PR review process

After you open your PR and mark it "ready for review", a member of our team will aim to review it within about a week. This
may be a bit longer if we're on holiday or tackling a particularly involved issue. Aside from sign-off and the automatic
CI checks, we also try to look for things like:

* Detailed comments and documentation
* Tests for new and existing code where possible
* A PR description that explains the decision tree behind the changes (what problem are you trying to solve, and why is
  this the right way to solve it? What else did you consider, but discounted?)
* Easy to read code, supported by comments

## Local development

With some effort around setting up a proper HTTPS federating server, it's possible to run policyserv from within your IDE.

If you have Docker installed, you can run `docker compose build && docker compose up` in the root of the repository to get
two Synapses and a policyserv instance running locally. The services are accessible at:

* Synapse (postgres): `localhost:4640`
* Synapse (sqlite): `localhost:4641`
* policyserv: `localhost:4642` (HTTPS at `localhost:4643`)

To ensure it's working, run `./dev/demo/sanity_check.sh` which will attempt to get all 3 servers federating with each 
other. 

If you'd like to test local Synapse changes, build a replacement Docker image tagged as `matrixdotorg/synapse:latest`
then re-build and re-run the Docker Compose setup above.

## Sign off

In order to have a concrete record that your contribution is intentional
and you agree to license it under the same terms as the project's license, we've adopted the
same lightweight approach that the Linux Kernel
[submitting patches process](https://www.kernel.org/doc/html/latest/process/submitting-patches.html#sign-your-work-the-developer-s-certificate-of-origin>),
[Docker](https://github.com/docker/docker/blob/master/CONTRIBUTING.md), and many other
projects use: the DCO (Developer Certificate of Origin:
http://developercertificate.org/). This is a simple declaration that you wrote
the contribution or otherwise have the right to contribute it to Matrix:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

If you agree to this for your contribution, then all that's needed is to
include the line in your commit or pull request comment:

```
Signed-off-by: Your Name <your@email.example.org>
```

Git allows you to add this signoff automatically when using the `-s`
flag to `git commit`, which uses the name and email set in your
`user.name` and `user.email` git configs.

## Getting support

We strive to be as helpful as possible, but we're not always able to respond to questions in real time. If you're having
trouble getting started, please join [#policyserv:matrix.org](https://matrix.to/#/#policyserv:matrix.org) on Matrix to 
talk with other contributors, users, and our team. That room also uses policyserv and other safety tooling to help maintain
a healthy work environment for everyone.
