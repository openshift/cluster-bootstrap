// META
repo = "kubernetes-incubator/bootkube"

// CONFIG
org_whitelist = ['coreos', 'coreos-inc']
job_admins = ['colemickens', 'ericchiang', 'rithujohn191', 'rphillips']
user_whitelist = job_admins

// JOBS
job_name = "tku-2-release-hyperkube"

pipelineJob(job_name) {
  parameters {
    stringParam('BOOTKUBE_VERSION', 'origin/master', 'git reference to build')
    stringParam('KUBERNETES_VERSION', 'origin/master', 'git reference to build')
    booleanParam('PUSH_IMAGE', false)
  }
  definition {
    cpsScm {
      scm {
        git {
          remote {
            github("${repo}")
            refspec('+refs/heads/*:refs/remotes/origin/* +refs/pull/*:refs/remotes/origin/pr/*')
            credentials('github_userpass')
          }
          branch('${BOOTKUBE_VERSION}')
        }
      }
      scriptPath('hack/jenkins/pipelines/hyperkube-release/Jenkinsfile')
    }
  }
}
