pipeline {
  agent {
    label 'slave'
  }

  options {
      timestamps()
      parallelsAlwaysFailFast()
      disableConcurrentBuilds()
      buildDiscarder(logRotator(numToKeepStr: '10', artifactNumToKeepStr: '10'))
  }

  stages {
    stage('Approval') {
      when {
        anyOf {
          branch pattern: "release/[\\w.]+", comparator: "REGEXP"
          branch  "main"
          branch  "deployment"
          buildingTag()
          changeRequest branch: 'release/[\\w.]+', comparator: "REGEXP"
        }
      }
      options {
        timeout(time: 1, unit: 'DAYS')
      }

      steps {
        script {
          user_inp = input id: 'deployer', message: 'Select deploy env', parameters: [
            choice(choices: ["staging", "prod", "None"], name: 'deploy_env')
          ]
          env.DEPLOY_TO = user_inp
        }
      }
    }

    stage('Deploy (staging)') {
      when {
        environment name: 'DEPLOY_TO', value: 'staging'
      }
      options {
        withAWS(credentials: 'AWS-Credentials-Stage', region: 'ap-south-1')
      }
      steps {
        dir("deploy/ansible") {
          s3Download(file: './config.toml', bucket: 'stage-ecs-env-myop', path: 'hookrelay/config.toml', force: true)
          sh "ansible-playbook -i inventory/staging.aws_ec2.yml playbook.yml"
        }
      }

      post {
          success {
            slackSend channel: 'jenkins-stage', message: "BLUE deployment complete: `${currentBuild.fullDisplayName}` (<${env.BUILD_URL}|Open>)", color: 'good'
          }
          failure {
            slackSend channel: 'jenkins-stage', message: "BLUE deployment failed: `${currentBuild.fullDisplayName}` (<${env.BUILD_URL}|Open>)", color: 'danger'
          }
          aborted {
            slackSend channel: 'jenkins-stage', message: "BLUE deployment cancelled: `${currentBuild.fullDisplayName}` (<${env.BUILD_URL}|Open>)", color: 'good'
          }
      }
    }


    stage('Deploy (prod)') {
      when {
        environment name: 'DEPLOY_TO', value: 'prod'
      }
      options {
        withAWS(credentials: 'AWS-Credentials', region: 'ap-south-1')
      }
      steps {
        dir("deploy/ansible") {
          s3Download(file: './config.toml', bucket: 'prod-ecs-env-myop', path: 'hookrelay/config.toml', force: true)
          sh "ansible-playbook -i inventory/prod.aws_ec2.yml playbook.yml"
        }
      }

      post {
          success {
            slackSend channel: 'jenkins-stage', message: "BLUE deployment complete: `${currentBuild.fullDisplayName}` (<${env.BUILD_URL}|Open>)", color: 'good'
          }
          failure {
            slackSend channel: 'jenkins-stage', message: "BLUE deployment failed: `${currentBuild.fullDisplayName}` (<${env.BUILD_URL}|Open>)", color: 'danger'
          }
          aborted {
            slackSend channel: 'jenkins-stage', message: "BLUE deployment cancelled: `${currentBuild.fullDisplayName}` (<${env.BUILD_URL}|Open>)", color: 'good'
          }
      }
    }

  }

  post {
    aborted {
      slackSend channel: 'jenkins-stage', message: "Job Cancelled: `${currentBuild.fullDisplayName}` | <${env.BUILD_URL}|Open>", color: '#e8e6e3'
    }

    failure {
      slackSend channel: 'jenkins-stage', message: "Job Failed: `${currentBuild.fullDisplayName}` | <${env.BUILD_URL}|Open>", color: 'danger'
    }
    success {
        slackSend channel: 'jenkins-stage', message: """
          Build succeeded `${currentBuild.fullDisplayName}`. (<${env.BUILD_URL}|Open>) (<${env.CHANGE_URL ?: env.GIT_URL}|Repo>)
          """, color: 'good'
    }
    always {
      cleanWs()
    }
  }
}
