<?php

class MySQL {

	private $handle = null;
	private $error = null;

	public function __construct($host, $port, $username, $password, $database) {
		try {
			$this->handle = new PDO("mysql:host=$host;port=$port;dbname=$database", $username, $password);
		}
		catch(PDOException $e) {
			// Handle this ourselves
			// stack traces on failed connections can include passwords
			$this->error['code'] = $e->getCode();
			$this->error['message'] = $e->getMessage();
		}

	}

	public function query($query, $data = null) {
		$qh = $this->handle->prepare($query);

		if ($data) {
			$qh->execute($data);
		} else {
			$qh->execute();
		}

		if ($qh->errorInfo()[1]) { echo "ERROR: ".$qh->errorInfo()[2]."\n"; }

		return $qh;
	}

	public function getHandle() {
		return $this->handle;
	}

	public function fetch($qh) {
		$qh->setFetchMode(PDO::FETCH_ASSOC);
		return $qh->fetch();
	}

	public function getInsertID() {
		return $this->handle->lastInsertId();
	}

	public function getError() {
		return $this->error;
	}

}
