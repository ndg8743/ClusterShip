// Jenkinsfile for ClusterShip
// Full CI/CD pipeline with linting, testing, building, Docker, and K8s integration

pipeline {
    agent any

    options {
        // Keep builds for 30 days or last 10 builds
        buildDiscarder(logRotator(numToKeepStr: '10', daysToKeepStr: '30'))

        // Timestamps in console output
        timestamps()

        // Timeout for entire pipeline
        timeout(time: 1, unit: 'HOURS')

        // Don't allow concurrent builds
        disableConcurrentBuilds()
    }

    environment {
        // Go configuration
        GO_VERSION = '1.24'
        GOPATH = "${WORKSPACE}/go"
        PATH = "${GOPATH}/bin:${PATH}"

        // Project configuration
        PROJECT_NAME = 'clustership'
        DOCKER_REGISTRY = 'ghcr.io'
        DOCKER_IMAGE = "${DOCKER_REGISTRY}/${GIT_REPO_OWNER}/${PROJECT_NAME}"

        // Versioning
        VERSION = sh(script: "git describe --tags --always --dirty 2>/dev/null || echo 'dev'", returnStdout: true).trim()
        COMMIT = sh(script: "git rev-parse --short HEAD", returnStdout: true).trim()
        BUILD_TIME = sh(script: "date -u '+%Y-%m-%d_%H:%M:%S'", returnStdout: true).trim()

        // Kubernetes
        K8S_NAMESPACE = 'clustership'

        // Tool versions
        GOLANGCI_LINT_VERSION = 'v1.61.0'

        // Credentials
        DOCKER_CREDENTIALS_ID = 'docker-registry-credentials'
        SLACK_CREDENTIALS_ID = 'slack-webhook'
        CODECOV_TOKEN_ID = 'codecov-token'
    }

    stages {
        stage('Setup') {
            steps {
                script {
                    echo "======================================"
                    echo "ClusterShip CI/CD Pipeline"
                    echo "======================================"
                    echo "Version: ${VERSION}"
                    echo "Commit: ${COMMIT}"
                    echo "Branch: ${env.GIT_BRANCH}"
                    echo "Build: ${env.BUILD_NUMBER}"
                    echo "======================================"
                }

                // Clean workspace
                cleanWs()

                // Checkout code
                checkout scm

                // Install Go
                sh """
                    echo "Setting up Go ${GO_VERSION}..."
                    mkdir -p ${GOPATH}/bin
                    go version || (curl -OL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz && \
                                   tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz)
                    go version
                """

                // Install golangci-lint
                sh """
                    echo "Installing golangci-lint ${GOLANGCI_LINT_VERSION}..."
                    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
                        sh -s -- -b ${GOPATH}/bin ${GOLANGCI_LINT_VERSION}
                    golangci-lint --version
                """
            }
        }

        stage('Dependencies') {
            steps {
                echo "Downloading dependencies..."
                sh '''
                    go mod download
                    go mod verify
                    go mod tidy
                '''
            }
        }

        stage('Lint') {
            steps {
                echo "Running golangci-lint..."
                sh '''
                    golangci-lint run --timeout=5m --config=.golangci.yml --out-format=checkstyle > golangci-lint-report.xml || true
                    golangci-lint run --timeout=5m --config=.golangci.yml
                '''
            }
            post {
                always {
                    // Record lint results
                    recordIssues(
                        enabledForFailure: true,
                        tools: [checkStyle(pattern: 'golangci-lint-report.xml')],
                        qualityGates: [[threshold: 1, type: 'TOTAL', unstable: true]]
                    )
                }
            }
        }

        stage('Test') {
            parallel {
                stage('Unit Tests') {
                    steps {
                        echo "Running unit tests with race detector..."
                        sh '''
                            mkdir -p coverage
                            go test -race -coverprofile=coverage/coverage.out -covermode=atomic ./... > test-results.txt
                            go tool cover -func=coverage/coverage.out | tee coverage/coverage.txt
                            go tool cover -html=coverage/coverage.out -o coverage/coverage.html
                        '''
                    }
                    post {
                        always {
                            // Archive test results
                            archiveArtifacts artifacts: 'test-results.txt,coverage/**', allowEmptyArchive: true

                            // Publish HTML coverage report
                            publishHTML(target: [
                                allowMissing: false,
                                alwaysLinkToLastBuild: true,
                                keepAll: true,
                                reportDir: 'coverage',
                                reportFiles: 'coverage.html',
                                reportName: 'Coverage Report'
                            ])
                        }
                        success {
                            echo "Unit tests passed"
                        }
                        failure {
                            echo "Unit tests failed"
                        }
                    }
                }

                stage('Code Quality') {
                    steps {
                        echo "Running go vet..."
                        sh 'go vet ./...'

                        echo "Checking formatting..."
                        sh '''
                            gofmt -l . > format-issues.txt || true
                            if [ -s format-issues.txt ]; then
                                echo "Files need formatting:"
                                cat format-issues.txt
                                exit 1
                            fi
                        '''
                    }
                }
            }
        }

        stage('Upload Coverage') {
            when {
                branch 'main'
            }
            steps {
                script {
                    withCredentials([string(credentialsId: env.CODECOV_TOKEN_ID, variable: 'CODECOV_TOKEN')]) {
                        sh '''
                            curl -Os https://uploader.codecov.io/latest/linux/codecov
                            chmod +x codecov
                            ./codecov -f coverage/coverage.out -t ${CODECOV_TOKEN}
                        '''
                    }
                }
            }
        }

        stage('Build') {
            parallel {
                stage('Build Linux AMD64') {
                    steps {
                        sh '''
                            echo "Building for Linux AMD64..."
                            GOOS=linux GOARCH=amd64 go build \
                                -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
                                -o bin/clustership_linux_amd64 \
                                ./cmd/clustership
                        '''
                    }
                }

                stage('Build Linux ARM64') {
                    steps {
                        sh '''
                            echo "Building for Linux ARM64..."
                            GOOS=linux GOARCH=arm64 go build \
                                -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
                                -o bin/clustership_linux_arm64 \
                                ./cmd/clustership
                        '''
                    }
                }

                stage('Build Darwin AMD64') {
                    steps {
                        sh '''
                            echo "Building for macOS AMD64..."
                            GOOS=darwin GOARCH=amd64 go build \
                                -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
                                -o bin/clustership_darwin_amd64 \
                                ./cmd/clustership
                        '''
                    }
                }

                stage('Build Darwin ARM64') {
                    steps {
                        sh '''
                            echo "Building for macOS ARM64..."
                            GOOS=darwin GOARCH=arm64 go build \
                                -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
                                -o bin/clustership_darwin_arm64 \
                                ./cmd/clustership
                        '''
                    }
                }

                stage('Build Windows AMD64') {
                    steps {
                        sh '''
                            echo "Building for Windows AMD64..."
                            GOOS=windows GOARCH=amd64 go build \
                                -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
                                -o bin/clustership_windows_amd64.exe \
                                ./cmd/clustership
                        '''
                    }
                }
            }
            post {
                success {
                    echo "Build artifacts created"
                    sh 'ls -lh bin/'
                    archiveArtifacts artifacts: 'bin/*', fingerprint: true
                }
            }
        }

        stage('Docker Build') {
            parallel {
                stage('Docker Standard') {
                    steps {
                        script {
                            echo "Building standard Docker image..."
                            sh """
                                docker build \
                                    --build-arg VERSION=${VERSION} \
                                    --build-arg COMMIT=${COMMIT} \
                                    -t ${DOCKER_IMAGE}:${VERSION} \
                                    -t ${DOCKER_IMAGE}:latest \
                                    .
                            """

                            // Security scan with Trivy
                            sh """
                                docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
                                    aquasec/trivy image --severity HIGH,CRITICAL \
                                    ${DOCKER_IMAGE}:${VERSION} || true
                            """
                        }
                    }
                }

                stage('Docker GPU') {
                    steps {
                        script {
                            echo "Building Docker image with GPU support..."
                            sh """
                                docker build \
                                    --build-arg VERSION=${VERSION} \
                                    --build-arg COMMIT=${COMMIT} \
                                    --build-arg GPU_SUPPORT=true \
                                    -t ${DOCKER_IMAGE}:${VERSION}-gpu \
                                    -t ${DOCKER_IMAGE}:latest-gpu \
                                    .
                            """
                        }
                    }
                }
            }
        }

        stage('K8s Integration Tests') {
            when {
                anyOf {
                    branch 'main'
                    changeRequest()
                }
            }
            steps {
                script {
                    echo "Setting up Kind cluster for integration tests..."
                    sh '''
                        # Install Kind if not present
                        if ! command -v kind &> /dev/null; then
                            curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
                            chmod +x ./kind
                            mv ./kind ${GOPATH}/bin/kind
                        fi

                        # Create Kind cluster
                        kind create cluster --name clustership-test || true

                        # Wait for cluster to be ready
                        kubectl cluster-info --context kind-clustership-test
                        kubectl wait --for=condition=Ready nodes --all --timeout=120s

                        # Run integration tests
                        go test -v -tags=integration ./pkg/k8s/...
                    '''
                }
            }
            post {
                always {
                    echo "Cleaning up Kind cluster..."
                    sh 'kind delete cluster --name clustership-test || true'
                }
                success {
                    echo "K8s integration tests passed"
                }
                failure {
                    echo "K8s integration tests failed"
                    sh 'kubectl get all -A || true'
                }
            }
        }

        stage('Benchmarks') {
            when {
                anyOf {
                    branch 'main'
                    expression { env.GIT_COMMIT_MESSAGE?.contains('[benchmark]') }
                }
            }
            steps {
                echo "Running performance benchmarks..."
                sh '''
                    mkdir -p benchmarks
                    go test -bench=. -benchmem -benchtime=10s -run=^$$ ./... | tee benchmarks/benchmark.txt
                '''
            }
            post {
                always {
                    archiveArtifacts artifacts: 'benchmarks/*.txt', allowEmptyArchive: true
                }
            }
        }

        stage('Docker Push') {
            when {
                anyOf {
                    branch 'main'
                    tag pattern: 'v\\d+\\.\\d+\\.\\d+', comparator: 'REGEXP'
                }
            }
            steps {
                script {
                    echo "Pushing Docker images to registry..."
                    withCredentials([usernamePassword(
                        credentialsId: env.DOCKER_CREDENTIALS_ID,
                        usernameVariable: 'DOCKER_USER',
                        passwordVariable: 'DOCKER_PASS'
                    )]) {
                        sh """
                            echo \$DOCKER_PASS | docker login ${DOCKER_REGISTRY} -u \$DOCKER_USER --password-stdin

                            # Push standard image
                            docker push ${DOCKER_IMAGE}:${VERSION}
                            docker push ${DOCKER_IMAGE}:latest

                            # Push GPU image
                            docker push ${DOCKER_IMAGE}:${VERSION}-gpu
                            docker push ${DOCKER_IMAGE}:latest-gpu

                            docker logout ${DOCKER_REGISTRY}
                        """
                    }
                }
            }
        }

        stage('Create Release') {
            when {
                tag pattern: 'v\\d+\\.\\d+\\.\\d+', comparator: 'REGEXP'
            }
            steps {
                script {
                    echo "Creating release archives..."
                    sh '''
                        mkdir -p release
                        cd bin
                        for binary in clustership_*; do
                            tar -czf ../release/${binary}.tar.gz ${binary}
                        done
                        cd ../release
                        ls -lh
                        sha256sum * > SHA256SUMS
                    '''
                }
            }
            post {
                success {
                    archiveArtifacts artifacts: 'release/*', fingerprint: true
                }
            }
        }
    }

    post {
        always {
            echo "Pipeline execution completed"

            // Clean up workspace (optional)
            // cleanWs()
        }

        success {
            script {
                def duration = currentBuild.durationString.replace(' and counting', '')
                echo "Pipeline succeeded in ${duration}"

                // Send Slack notification on success (if configured)
                if (env.SLACK_CREDENTIALS_ID) {
                    slackSend(
                        color: 'good',
                        message: """
                            :white_check_mark: ClusterShip Build Successful

                            *Branch:* ${env.GIT_BRANCH}
                            *Commit:* ${COMMIT}
                            *Version:* ${VERSION}
                            *Duration:* ${duration}
                            *Build:* ${env.BUILD_URL}
                        """.stripIndent(),
                        channel: '#builds',
                        tokenCredentialId: env.SLACK_CREDENTIALS_ID
                    )
                }
            }
        }

        failure {
            script {
                def duration = currentBuild.durationString.replace(' and counting', '')
                echo "Pipeline failed in ${duration}"

                // Send Slack notification on failure (if configured)
                if (env.SLACK_CREDENTIALS_ID) {
                    slackSend(
                        color: 'danger',
                        message: """
                            :x: ClusterShip Build Failed

                            *Branch:* ${env.GIT_BRANCH}
                            *Commit:* ${COMMIT}
                            *Duration:* ${duration}
                            *Build:* ${env.BUILD_URL}

                            Please check the build logs for details.
                        """.stripIndent(),
                        channel: '#builds',
                        tokenCredentialId: env.SLACK_CREDENTIALS_ID
                    )
                }
            }
        }

        unstable {
            echo "Pipeline completed with warnings"
        }

        aborted {
            echo "Pipeline was aborted"
        }
    }
}
