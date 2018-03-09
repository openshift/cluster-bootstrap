# hack/jenkins

Job configurations, pipelines, scripts, and docker images used for `bootkube` validation.

### Overvew

This contains all specifications for `bootkube` Jenkins jobs. No changes are made to these jobs
after they are deployed. They are re-deployed each time Jenkins is restarted. The goal is for
Jenkins and its job configurations to be as fungible as possible.

### Jenkins Quick Information

-   Declarative, Verison-Controlled Jenkins Jobs
    -   (via [Job DSL Plugin](https://github.com/jenkinsci/job-dsl-plugin))
    -   It defines a groovy DSL for specifying Jenkins job in a declarative-looking imperative syntax that emits Jenkins job xml.
    -   It provides a "Build Action" that is used in a "Seed Job" (defined in a separate repo) to instantiate jobs.
    -   Jobs for this repo are defined in `./jobs/`.
-   Jenkinsfile / Jenkins Pipelines
    -   (built-in to Jenkins)
    -   Jenkins has pipelines for defining workflows for jobs in a version-control friendly manner (as opposed to the non-friendly XML files that it uses internally)
    -   Confusingly, they come in two variants - [Declarative Pipelines](https://jenkins.io/doc/book/pipeline/syntax/#declarative-pipeline) and [Scripted Pipelines](https://jenkins.io/doc/book/pipeline/syntax/#scripted-pipeline).
        Be sure you are reading the docs for the right kind. They have slightly different steps, and even they have same-named steps, they can have behavioral differences.
    -   This repository favors "Declarative Pipelines" where possible.
        -   It seems to be where Jenkins is trending and trying to push people toward.
        -   The syntax is more convincingly declarative.
        -   It has an escape hatch that allows you to enter a scripting block and use the full scripting syntax.
    -   Pipelines for this repo are defined in `./pipelines/`.
-   Kubernetes Plugin
    -   (via [Kubernetes Plugin](https://github.com/jenkinsci/kubernetes-plugin))
    -   Allows using a Kubernetes cluster as a Jenkins "Cloud" (resource from which worker Agents can be spawned).
    -   _Note:_ All jobs here are expected to run on the default `kubernetes` cloud.

### Structure

-   `images`: Contains `Dockerfile`s for any images used in validation. (_Note:_ images used for CI are not considered or supported as part of any bootkube release.)
-   `jobs`: Contains top-level groovy scripts containing the Job DSL configurations. (these are recommeneded to be (but are not necessarily) Pipeline Jobs.)
-   `pipelines`: Contains `Jenkinsfile` pipelines used as part of pipeline jobs.
-   `scripts`: Contains scripts that are used in Pipelines, or in Jobs directly. (May or may not be usable outside of Jenkins. Only supported as part of CI.)

This shows an example directory structure, added as part of the `bootkube-e2e-*` jobs:

    hack/jenkins
    ├── images
    │   └── bootkube-e2e
    │       └── Dockerfile
    ├── jobs
    │   └── bootkube_e2e.groovy
    ├── pipelines
    │   └── bootkube-e2e
    │       └── Jenkinsfile
    └── scripts
        ├── e2e.sh
        ├── tqs-down.sh
        └── tqs-up.sh

### Currently Defined Jobs

-   bootkube-e2e-\*
    -   calico: tests a standard single-master bootkube cluster with calico
    -   flannel: tests a standard single-master bootkube cluster with flannel
