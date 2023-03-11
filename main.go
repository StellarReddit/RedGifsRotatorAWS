package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/StellarReddit/RedGifsWrapper"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/google/uuid"
)

// These variables are injected at compile time
var (
	RedGifsClientID     string
	RedGifsClientSecret string
	RedGifsTestID       string
)

const serverUserAgent = "app.stellarreddit.RedGifsServer (email: legal@azimuthcore.com)"

func main() {
	lambda.Start(CredentialRotator)
}

// CredentialRotator - request a new access token, validate, and perform rotation
func CredentialRotator(_ context.Context) {
	// Create the RedGifs client
	redGifsClient := RedGifsWrapper.NewClient(RedGifsWrapper.Config{
		ClientID:     RedGifsClientID,
		ClientSecret: RedGifsClientSecret,
		UserAgent:    serverUserAgent,
	})

	backoff := []time.Duration{3 * time.Second, 5 * time.Second, 10 * time.Second}

	for _, v := range backoff {
		accessToken, refreshErr := redGifsClient.RequestNewAccessToken()

		if refreshErr != nil {
			time.Sleep(v)
			continue
		}

		time.Sleep(3 * time.Second) // Wait 3 seconds for token to be active

		_, streamErr := redGifsClient.LookupStreamURL("", serverUserAgent, RedGifsTestID, accessToken)

		// Success if no error, or if streamErr that is 404/410.
		if streamErr == nil || errors.Is(streamErr, RedGifsWrapper.ErrNotFound) {
			rotateAWSSecret(accessToken)
			break
		}

		// Otherwise, try up to 2 more times
		time.Sleep(v)
	}
}

// rotateAWSSecret - rotate the RedGifs access token
func rotateAWSSecret(newAccessToken string) {
	sess := session.Must(session.NewSession())
	client := secretsmanager.New(sess)
	secretID := aws.String("s4r-redgifs-accesstoken")

	// Store the secret
	_, err := client.PutSecretValueWithContext(context.Background(), &secretsmanager.PutSecretValueInput{
		ClientRequestToken: aws.String(uuid.NewString()),
		SecretId:           secretID,
		SecretString:       aws.String(newAccessToken),
	})

	// Do nothing if error, token will be valid for 2 more weeks anyway
	if err != nil {
		fmt.Println(err)
	}
}
