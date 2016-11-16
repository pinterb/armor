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
  //
  // tag image with version, and branch-commit-id
  //
  // compile tag list
  //

  stage ('preparation') {
    sh "mkdir -p ${workDir}"
    sh "cp -R ${pwd}/* ${workDir}"
    sh "echo golang: $(which go)"
  }
}
