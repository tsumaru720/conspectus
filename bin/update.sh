#!/bin/bash

HAS_COMPOSER=$(which composer)
COMPOSER=""

if [ "$HAS_COMPOSER" != "" ]; then
	COMPOSER="composer"
else
	if [ -f "composer.phar" ]; then
		COMPOSER="./composer.phar"
	fi

	if [ -f "../composer.phar" ]; then
		COMPOSER="../../composer.phar"
	fi

	if [ -f "../../composer.phar" ]; then
		COMPOSER="../../composer.phar"
	fi

	if [ "$COMPOSER" == "" ]; then
		echo "Unable to find composer"
		exit;
	fi
fi

git pull
$COMPOSER install -d "../"
php db.php
