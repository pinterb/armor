#!/usr/bin/groovy

// load pipeline functions
@Library('github.com/pinterb/jenkins-pipeline@master')
def pipeline = new io.clusterops.Pipeline()

node {
  def goPath = "/go"
  def workDir = "${goPath}/src/github.com/pinterb/armor"
  def pwd = pwd()

  checkout scm

  // read in required jenkins pipeline configuration values
  def inputFile = readFile('Jenkinsfile.json')
  // NOTE: add the following to jenkins 'In-process Script Approval':
  //   method groovy.json.JsonSlurperClassic parseText java.lang.String
  //   new groovy.json.JsonSlurperClassic
  def config = new groovy.json.JsonSlurperClassic().parseText(inputFile)
  println "pipeline config ==> ${config}"

  // continue only if pipeline enabled
  if (!config.pipeline.enabled) {
    println "pipeline disabled...quitting build now"
    return
  }

  // set additional git envars for image tagging
  pipeline.gitEnvVars()

  // used to debug deployment setup
  env.DEBUG_DEPLOY = false

  // debugging helm deployments
  if (env.DEBUG_DEPLOY) {
    println "Runing helm tests"
    pipeline.kubectlTest()
    pipeline.helmConfig()
  }

  def acct = pipeline.getContainerRepoAcct(config)

  // tag image with version, and branch-commit-id
  def image_tags_map = pipeline.getContainerTags(config)

  // compile tag list
  def image_tags_list = pipeline.getMapValues(image_tags_map)

stage ('preparation') {
    // Print env -- debugging
    sh "env | sort"

    sh "mkdir -p ${workDir}"
    sh "cp -R ${pwd}/* ${workDir}"
    sh "go version"
  }
}
