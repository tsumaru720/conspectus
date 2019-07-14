<?php

class Document extends Theme {

	protected $pageTitle = 'Analytics';
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

		$vars['single_asset'] = false;

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
					$this->pageTitle = "Analytics - ".$item['description'];
					$vars['page_title'] = $item['description'];
					$vars['asset_class'] = $item['class'];
					$vars['single_asset'] = true;
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
					$this->pageTitle = "Analytics - ".$item['description'];
					$vars['page_title'] = $item['description'];
					if ($item['count'] == 1) {
						$vars['single_asset'] = true;
					}
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

		while ($period = $this->db->fetch($dataQuery)) {
			$period['value_delta'] = 0;
			$period['growth_factor'] = 0;

			if (isset($last)) {
				$period['value_delta'] = number_format($period['deposit_total'] - $last['deposit_total'], 2, '.', '');

				if (($period['asset_total'] != 0) && ($last['asset_total'] != 0)) {
					$period['growth_factor'] = $period['asset_total'] / ($last['asset_total'] + $period['value_delta']);
				}
			}

			if ($period['year'] == date("Y")) {
				$period['year'] = "YTD";
			}

			if (!isset($vars['periodData'][$period['year']])) {
				$vars['periodData'][$period['year']]['label'] = $period['year'];
				$vars['periodData'][$period['year']]['value'] = 0;
			}

			if ($vars['periodData'][$period['year']]['value'] == 0) {
				$vars['periodData'][$period['year']]['value'] = $period['growth_factor'];
			} else {
				$vars['periodData'][$period['year']]['value'] = $vars['periodData'][$period['year']]['value'] * $period['growth_factor'];
			}

			$last = $period;
		}


		foreach ($vars['periodData'] as $key => $value) {
			if ($value['value'] != 0) {
				$value['value'] = ($value['value'] - 1) * 100;
				$vars['periodData'][$key]['value'] = number_format($value['value'],2);
			}
		}

		$this->vars = $vars;
		$this->document = $twig->load('analytics.html');
	}


}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
