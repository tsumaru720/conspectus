<?php

class Document extends Theme {

	protected $pageTitle = 'Dashboard';
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
			                                SUM(asset_value - deposit_value) AS gain,
			                                DATE_FORMAT(epoch, '%b %Y') AS period,
			                                EXTRACT(YEAR_MONTH
			                            FROM
			                                epoch) AS yearMonth
			                            FROM
			                                asset_log
			                            WHERE
			                                asset_id ".$vars['modifier']." :item_id
			                            GROUP BY
			                                period,
			                                yearMonth
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
					$this->pageTitle = "Asset View - ".$item['description'];
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
			                                SUM(asset_value - deposit_value) AS gain,
			                                DATE_FORMAT(epoch, '%b %Y') AS period,
			                                EXTRACT(YEAR_MONTH
			                            FROM
			                                epoch) AS yearMonth
			                            FROM
			                                asset_log
			                            LEFT JOIN asset_list ON asset_log.asset_id = asset_list.id
			                            LEFT JOIN asset_classes ON asset_classes.id = asset_list.asset_class
			                            WHERE
			                                asset_classes.id ".$vars['modifier']." :item_id
			                            GROUP BY
			                                period,
			                                yearMonth
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
					$this->pageTitle = "Class View - ".$item['description'];
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

		while ($period = $this->db->fetch($dataQuery)) {
			$period['gain_delta'] = 0;
			$period['value_delta'] = 0;
			$period['growth_annualized'] = 0;
			$period['growth_factor'] = 0;
			$period['twr'] = 0;
			$period['twr_str'] = 0;

			if (isset($last)) {
				$period['gain_delta'] = number_format($period['gain'] - $last['gain'], 2, '.', '');
				$period['value_delta'] = number_format($period['deposit_total'] - $last['deposit_total'], 2, '.', '');

				if (($period['asset_total'] != 0) && ($last['asset_total'] != 0)) {
					$period['growth_factor'] = $period['asset_total'] / ($last['asset_total'] + $period['value_delta']);
					if ($last['twr'] == 0) {
						$period['twr'] = $period['growth_factor'];
					} else {
						$period['twr'] = $last['twr'] * $period['growth_factor'];
					}
					if ($period['twr'] != 0) {
						$period['twr_str'] = ($period['twr'] - 1) * 100;
						$period['twr_str'] = number_format($period['twr_str'],2);
					}
				}

				if ($last['asset_total'] != 0) {
					$period['growth_annualized'] = pow($period['growth_factor'], 12) - 1;
					$period['growth_annualized'] = number_format($period['growth_annualized'] * 100, 2, '.', '');
				}
			}

			if ($period['deposit_total'] != 0) {
				$period['growth'] = number_format(($period['gain'] / $period['deposit_total']) * 100, 2, '.', '');
			} else {
				if ($period['gain'] > 0) {
					$period['growth'] = 100;
				} else {
					$period['growth'] = 0;
				}
			}
			$period['growth_str'] = number_format($period['growth'],2);
			$vars['periodData'][] = $period;
			$last = $period;
		}

		if (isset($last)) {  // Probably no entries if this doesnt pass
			$last['deposit_str'] = $this->prettify($last['deposit_total']);
			$last['asset_str'] = $this->prettify($last['asset_total']);
			$last['gain_str'] = $this->prettify($last['gain']);
			$vars['mostRecent'] = $last;
		} else {
			$last['deposit_str'] = $this->prettify('0');
			$last['asset_str'] = $this->prettify('0');
			$last['gain_str'] = $this->prettify('0');
			$last['growth_str'] = '0.00';
			$last['twr_str'] = '0.00';
			$vars['mostRecent'] = $last;
		}

		//$this->setRegister('script', "https://cdnjs.cloudflare.com/ajax/libs/Chart.js/2.7.3/Chart.min.js");
		$this->vars = $vars;
		$this->document = $twig->load('item_view.html');
	}

	private function prettify($value) {
		return ($value < 0 ? '-£'.number_format($value * -1, 2) : '£'.number_format($value, 2));
		//TODO some kind of configurable currency
	}

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
