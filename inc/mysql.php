<?php

/*
$data = array('username' => 'test');
$query = $mysql->query("SELECT * from users where username = :username", $data);
var_dump($mysql->fetch($query));
var_dump($mysql->fetch($query));
*/

class MySQL {

	public function __construct($host, $port, $username, $password, $database) {
		$this->host = $host;
		$this->port = $port;
		$this->username = $username;
		$this->password = $password;
		$this->database = $database;

		try {
			$this->handle = new PDO("mysql:host=$host;port=$port;dbname=$database", $username, $password);
		}
		catch(PDOException $e) { 
			return null;
		}

		return $this->handle;
	}

	public function query($query, $data = null) {
		$qh = $this->handle->prepare($query);

		if ($data) {
			$qh->execute($data);
		} else {
			$qh->execute();
		}
		return $qh;
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