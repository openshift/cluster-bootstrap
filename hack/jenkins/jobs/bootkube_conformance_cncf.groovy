// META
repo = "kubernetes-incubator/bootkube"

// CONFIG
org_whitelist = ['coreos', 'coreos-inc']
job_admins = ['ericchiang', 'rithujohn191', 'rphillips']
user_whitelist = job_admins

// JOBS
job_name = "tku-bootkube-conformance-cncf"

pipelineJob(job_name) {
  parameters {
    stringParam('sha1', 'master', 'git reference to build')
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
          branch('${sha1}')
        }
      }
      scriptPath('hack/jenkins/pipelines/bootkube-conformance/Jenkinsfile')
    }
    triggers {
      cron('H 11 * * *')
    }
  }
}
