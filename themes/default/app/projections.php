<?php

class Document extends Theme {

	protected $pageTitle = 'Projections';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();

		if ($vars['left_menu'] != 'all') {
			if (!is_numeric($vars['item_id']) || $vars['item_id'] < 0) {
				echo "bad id";
				die();
				//TODO make this error nicer
			} else {
				$vars['modifier'] = "=";
			}
		} else {
			$vars['item_id'] = 0;
			$vars['modifier'] = ">";
		}
		$data = array(':item_id' => $vars['item_id']);

		if ($vars['type'] == 'asset') {
			$dataQuery = $this->db->query("SELECT
			                                SUM(deposit_value) AS deposit_total,
			                                SUM(asset_value) AS asset_total,
			                                DATE_FORMAT(epoch, '%b %Y') AS period,
			                                EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
			                                EXTRACT(YEAR FROM epoch) AS year
			                            FROM
			                                asset_log
			                            WHERE
			                                asset_id ".$vars['modifier']." :item_id
			                            GROUP BY
			                                period,
			                                yearMonth,
			                                year
			                            ORDER BY
			                                yearMonth ASC", $data);
			if ($vars['item_id'] > 0) {
				$nameQuery = $this->db->query("SELECT
				                                asset_list.description,
				                                asset_classes.description AS class
				                            FROM
				                                asset_list
				                            LEFT JOIN asset_classes ON asset_class = asset_classes.id
				                            WHERE
				                                asset_list.id = :item_id;", $data);
				if ($item = $this->db->fetch($nameQuery)) {
					$this->pageTitle = "Projections - ".$item['description'];
					$vars['page_title'] = $item['description'];
					$vars['asset_class'] = $item['class'];
				} else {
					echo "invalid asset";
					die();
					//TODO make this error nicer
				}
			}
		} elseif ($vars['type'] == 'class') {
	 		$dataQuery = $this->db->query("SELECT
			                                SUM(deposit_value) AS deposit_total,
			                                SUM(asset_value) AS asset_total,
			                                DATE_FORMAT(epoch, '%b %Y') AS period,
			                                EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth,
			                                EXTRACT(YEAR FROM epoch) AS year
			                            FROM
			                                asset_log
			                            LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
			                            LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
			                            WHERE
			                                asset_classes.id ".$vars['modifier']." :item_id
			                            GROUP BY
			                                period,
			                                yearMonth,
			                                year
			                            ORDER BY
			                                yearMonth ASC", $data);
			if ($vars['item_id'] > 0) {
				$nameQuery = $this->db->query("SELECT
				                                count(asset_list.description) as count,
				                                asset_classes.description AS description
				                            FROM
				                                asset_list
				                            LEFT JOIN asset_classes ON asset_class = asset_classes.id
				                            WHERE
				                                asset_classes.id = :item_id;", $data);
				$item = $this->db->fetch($nameQuery); // count() in the query will ensure this always exists
				if (($item['count'] > 0) && ($item['description'] != 'NULL')) {
					$this->pageTitle = "Projections - ".$item['description'];
					$vars['page_title'] = $item['description'];
				} else {
					echo "invalid class";
					die();
					//TODO make this error nicer
				}
			}
		} else {
			echo "invalid type";
			die();
			//TODO make this error nicer
		}

		$startValue = 0;
		$startYear = 0;
		$curYear = date("Y");

		$endYear = "2048";
		$endValue = "1000000";
		//TODO: make these optional

		while ($period = $this->db->fetch($dataQuery)) {
			$year = $period['year'];
			if ($startValue == 0) {
				$startYear = $year;
				$startValue = $period['asset_total'];
				$vars['startYear'] = $startYear;
				$vars['startValue'] = $startValue;
			}
			$vars['actual']['data'][$year]['year'] = $year;
			$vars['actual']['data'][$year]['value'] = $period['asset_total'];
		}

		$displayYears = ($curYear + 10) - $startYear;
		$vars['initial'] = $this->seedTargets($startYear, $endYear, $startValue, $endValue, $displayYears);
		$vars['revised'] = $this->seedTargets($curYear + 1, $endYear, $vars['actual']['data'][$curYear]['value'], $endValue, $displayYears);

		$revised_seed = array_fill(0, ($curYear - $startYear + 1), 0);
		$vars['revised']['data'] = array_merge($revised_seed, $vars['revised']['data']);

//var_dump($vars);

		$this->vars = $vars;
		$this->document = $twig->load('projections.html');
	}

	private function seedTargets($startYear, $endYear, $startValue, $endValue, $returnAmount = 0) {
		$years = $endYear - $startYear;
		$percent = pow(($endValue / $startValue), (1 / $years));
		$targets['percent'] = $percent;
		$targets['percent_str'] = number_format(($percent - 1) * 100, 2);

		if ($returnAmount == 0) {
			$returnAmount = $years;
		}

		$value = $startValue;
		for ($i = 0; $i < $years; $i++) {
			if ($i == $returnAmount) { break; }
			$value = $value * $percent;
			$targets['data'][$i]['year'] = $startYear + $i;
			$targets['data'][$i]['value'] = number_format($value, 2, '.', '');
		}
		return $targets;
	}

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
