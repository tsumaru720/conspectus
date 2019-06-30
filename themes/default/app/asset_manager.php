<?php

class Document extends Theme {

	protected $pageTitle = 'Asset Manager';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$this->db = $main->getDB();
		$this->vars = $vars;

		$q = $this->db->query("SELECT * from asset_classes ORDER BY description ASC");
		while ($item = $this->db->fetch($q)) {
			$this->vars['class_menu_items'][] = $item;
		}

		// Other requests types should never get this far
		// due to bramus router matching.
		if ($_SERVER['REQUEST_METHOD'] == "GET") {
			$this->processGet($twig, $vars['action']);
		} elseif ($_SERVER['REQUEST_METHOD'] == "POST") {
			$this->processPost($twig, $vars['action']);
		}
		
	}

	private function processGet(&$twig, $action) {
		if ($action == "new") {
			$this->pageTitle .= " - New Asset";
			$vars['page_title'] = $this->pageTitle;
			$this->document = $twig->load('asset_manager_new.html');
		} elseif ($action == "edit") {
			echo "not implemented yet";
			die();
		} elseif ($action == "delete") {
			echo "not implemented yet";
			die();
		} else {
			// Should probably handle this nicer
			// Chances are someone's trying to break something
			// So meh.
			die();
		}
	}

	private function processPost(&$twig, $action) {
		if ($action == "new") {
			$description = $this->validateInput('description');
			$class = $this->validateInput('class');

			$validated = true;
			if (($description == false) || $class == false) {
				$validated = false;
			}

			$data = array(':description' => $description);
			$q = $this->db->query("SELECT COUNT(id) as count from asset_list WHERE description = :description", $data);
			$result = $this->db->fetch($q);

			if ($result['count'] > 0) {
				// Name specified already exists - its an exact match
				// Probably dont want duplicates.
				$validated = false;
			}

			$data = array(':class_id' => $class);
			$q = $this->db->query("SELECT COUNT(id) as count from asset_classes WHERE id = :class_id", $data);
			$result = $this->db->fetch($q);
			
			if ($result['count'] == 0) {
				// ID provided doesnt exist.
				$class = false;
				$validated = false;
			}
			
			if ($validated == false) {
				$this->vars['error'] = true;
				$this->vars['form_description'] = $description;
				$this->vars['form_class'] = $class;
			} else {
				$this->vars['success'] = true;
				$data = array(':description' => $description, ':class_id' => $class);
				$q = $this->db->query("INSERT INTO `asset_list` (`id`, `asset_class`, `description`) VALUES (NULL, :class_id, :description)", $data);
			}

			$this->document = $twig->load('asset_manager_new.html');
		} elseif ($action == "edit") {
			echo "not implemented yet";
			die();
		} elseif ($action == "delete") {
			echo "not implemented yet";
			die();
		} else {
			// Should probably handle this nicer
			// Chances are someone's trying to break something
			// So meh.
			die();
		}
	}

	private function validateInput($key) {
		// Simple validation we can apply to everything
		if (!array_key_exists($key, $_POST)) {
			return false;
		} else {
			if ($_POST[$key] == "") {
				return false;
			}
		}
		return $_POST[$key];
	}

}
