pipeline {
    agent any

    options {
        buildDiscarder(logRotator(numToKeepStr: '10', daysToKeepStr: '30'))
        timestamps()
        timeout(time: 1, unit: 'HOURS')
        disableConcurrentBuilds()
    }

    environment {
        GO_VERSION = '1.24'
        GOPATH = "${WORKSPACE}/go"
        PATH = "${GOPATH}/bin:${PATH}"
        PROJECT_NAME = 'clustership'
        DOCKER_REGISTRY = 'ghcr.io'
        DOCKER_IMAGE = "${DOCKER_REGISTRY}/${GIT_REPO_OWNER}/${PROJECT_NAME}"
        VERSION = sh(script: "git describe --tags --always --dirty 2>/dev/null || echo 'dev'", returnStdout: true).trim()
        COMMIT = sh(script: "git rev-parse --short HEAD", returnStdout: true).trim()
        BUILD_TIME = sh(script: "date -u '+%Y-%m-%d_%H:%M:%S'", returnStdout: true).trim()
        K8S_NAMESPACE = 'clustership'
        GOLANGCI_LINT_VERSION = 'v1.61.0'
        DOCKER_CREDENTIALS_ID = 'docker-registry-credentials'
        CODECOV_TOKEN_ID = 'codecov-token'
    }

    stages {
        stage('Setup') {
            steps {
                cleanWs()
                checkout scm
                sh '''
                    mkdir -p ${GOPATH}/bin
                    go version
                    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH}/bin ${GOLANGCI_LINT_VERSION}
                '''
            }
        }

        stage('Dependencies') {
            steps {
                sh 'go mod download && go mod verify'
            }
        }

        stage('Lint') {
            steps {
                sh 'golangci-lint run --timeout=5m --config=.golangci.yml'
            }
        }

        stage('Test') {
            parallel {
                stage('Unit Tests') {
                    steps {
                        sh '''
                            mkdir -p coverage
                            go test -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
                            go tool cover -html=coverage/coverage.out -o coverage/coverage.html
                        '''
                    }
                    post {
                        always {
                            archiveArtifacts artifacts: 'coverage/**', allowEmptyArchive: true
                        }
                    }
                }
                stage('Vet') {
                    steps {
                        sh 'go vet ./...'
                    }
                }
            }
        }

        stage('Build') {
            parallel {
                stage('Linux AMD64') {
                    steps {
                        sh 'GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=${VERSION}" -o bin/clustership_linux_amd64 ./cmd/clustership'
                    }
                }
                stage('Linux ARM64') {
                    steps {
                        sh 'GOOS=linux GOARCH=arm64 go build -ldflags "-X main.Version=${VERSION}" -o bin/clustership_linux_arm64 ./cmd/clustership'
                    }
                }
                stage('Darwin AMD64') {
                    steps {
                        sh 'GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=${VERSION}" -o bin/clustership_darwin_amd64 ./cmd/clustership'
                    }
                }
                stage('Darwin ARM64') {
                    steps {
                        sh 'GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=${VERSION}" -o bin/clustership_darwin_arm64 ./cmd/clustership'
                    }
                }
                stage('Windows AMD64') {
                    steps {
                        sh 'GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=${VERSION}" -o bin/clustership_windows_amd64.exe ./cmd/clustership'
                    }
                }
            }
            post {
                success {
                    archiveArtifacts artifacts: 'bin/*', fingerprint: true
                }
            }
        }

        stage('Docker') {
            steps {
                sh "docker build --build-arg VERSION=${VERSION} -t ${DOCKER_IMAGE}:${VERSION} -t ${DOCKER_IMAGE}:latest ."
            }
        }

        stage('K8s Tests') {
            when { anyOf { branch 'main'; changeRequest() } }
            steps {
                sh '''
                    kind create cluster --name clustership-test || true
                    kubectl wait --for=condition=Ready nodes --all --timeout=120s
                    go test -v -tags=integration ./pkg/k8s/...
                '''
            }
            post {
                always {
                    sh 'kind delete cluster --name clustership-test || true'
                }
            }
        }

        stage('Push') {
            when { anyOf { branch 'main'; tag pattern: 'v\\d+\\.\\d+\\.\\d+', comparator: 'REGEXP' } }
            steps {
                withCredentials([usernamePassword(credentialsId: env.DOCKER_CREDENTIALS_ID, usernameVariable: 'DOCKER_USER', passwordVariable: 'DOCKER_PASS')]) {
                    sh '''
                        echo $DOCKER_PASS | docker login ${DOCKER_REGISTRY} -u $DOCKER_USER --password-stdin
                        docker push ${DOCKER_IMAGE}:${VERSION}
                        docker push ${DOCKER_IMAGE}:latest
                        docker logout ${DOCKER_REGISTRY}
                    '''
                }
            }
        }
    }

    post {
        success { echo "Build succeeded" }
        failure { echo "Build failed" }
    }
}
