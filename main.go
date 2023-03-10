package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/StellarReddit/RedGifsWrapper"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/google/uuid"
	"time"
)

// These variables are injected at compile time
var (
	RedGifsClientId     string
	RedGifsClientSecret string
	RedGifsTestId       string
)

const (
	ServerUserAgent = "app.stellarreddit.RedGifsServer (email: legal@azimuthcore.com)"
)

func main() {
	lambda.Start(CredentialRotator)
}

// CredentialRotator - request a new access token, validate, and perform rotation
func CredentialRotator(_ context.Context) {
	// Create the RedGifs client
	redGifsClient := RedGifsWrapper.NewClient(RedGifsWrapper.Config{
		ClientID:     RedGifsClientId,
		ClientSecret: RedGifsClientSecret,
		UserAgent:    ServerUserAgent,
	})

	backoff := [3]time.Duration{3, 5, 10}

	for _, v := range backoff {
		accessToken, refreshErr := redGifsClient.RequestNewAccessToken()

		if refreshErr != nil {
			time.Sleep(v * time.Second)
			continue
		}

		time.Sleep(3 * time.Second) // Wait 3 seconds for token to be active

		_, streamErr := redGifsClient.LookupStreamURL("", ServerUserAgent, RedGifsTestId, accessToken)

		// Success if no error, or if streamErr that is 404/410.
		if streamErr == nil || errors.Is(streamErr, RedGifsWrapper.ErrNotFound) {
			rotateAWSSecret(accessToken)
			break
		} else {
			// Otherwise, try up to 2 more times
			time.Sleep(v * time.Second)
		}
	}
}

// rotateAWSSecret - rotate the RedGifs access token
func rotateAWSSecret(newAccessToken string) {
	sess := session.Must(session.NewSession())
	client := secretsmanager.New(sess)
	secretId := aws.String("s4r-redgifs-accesstoken")

	// Store the secret
	_, err := client.PutSecretValue(&secretsmanager.PutSecretValueInput{
		ClientRequestToken: aws.String(uuid.NewString()),
		SecretId:           secretId,
		SecretString:       aws.String(newAccessToken),
	})

	// Do nothing if error, token will be valid for 2 more weeks anyway
	fmt.Println(err)
}
