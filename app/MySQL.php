<?php

class MySQL {

	private $handle = null;

	public function __construct($host, $port, $username, $password, $database) {
		try {
			$this->handle = new PDO("mysql:host=$host;port=$port;dbname=$database", $username, $password);
		}
		catch(PDOException $e) {
			return null; //do nothing
		}

	}

	public function query($query, $data = null) {
		$qh = $this->handle->prepare($query);

		if ($data) {
			$qh->execute($data);
		} else {
			$qh->execute();
		}

		if ($qh->errorInfo()) { echo $qh->errorInfo()[2]; }

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

}
?>
