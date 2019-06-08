<?php

class Document extends Theme {

	protected $pageTitle = 'Dashboard';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();
		$data = array(':item_id' => $vars['item_id']);

		if ($vars['nav_item'] == 'asset') {
			$dataQuery = $this->db->query("SELECT SUM(deposit_value) AS deposit_total,
									SUM(asset_value) AS asset_total,
									SUM(asset_value - deposit_value) AS gain,
									DATE_FORMAT(epoch, '%b %Y') AS period,
									EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
									FROM asset_log
									WHERE asset_id ".$vars['modifier']." :item_id
									GROUP BY period, yearMonth
									ORDER BY yearMonth ASC", $data);
			if ($vars['item_id'] > 0) {
				$nameQuery = $this->db->query("SELECT description FROM asset_list WHERE id = :item_id;", $data);
				if ($item = $this->db->fetch($nameQuery)) {
					$this->pageTitle = "Asset View - ".$item['description'];
					$vars['page_title'] = $this->pageTitle;
				} else {
					echo "invalid asset";
					die();
					//TODO make this error nicer
				}
			}
		} elseif ($vars['nav_item'] == 'class') {
	 		$dataQuery = $this->db->query("SELECT SUM(deposit_value) AS deposit_total,
	 								SUM(asset_value) AS asset_total,
	 								SUM(asset_value - deposit_value) AS gain,
	 								DATE_FORMAT(epoch, '%b %Y') AS period,
	 								EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
	 								FROM asset_log
	 								LEFT JOIN asset_list
	 								ON asset_log.asset_id = asset_list.id
	 								LEFT JOIN asset_classes
	 								ON asset_classes.id = asset_list.asset_class
	 								WHERE asset_classes.id ".$vars['modifier']." :item_id
	 								GROUP BY period, yearMonth
	 								ORDER BY yearMonth ASC", $data);
			if ($vars['item_id'] > 0) {
				$nameQuery = $this->db->query("SELECT description FROM asset_classes WHERE id = :item_id;", $data);
				if ($item = $this->db->fetch($nameQuery)) {
					$this->pageTitle = "Class View - ".$item['description'];
					$vars['page_title'] = $this->pageTitle;
				} else {
					echo "invalid asset";
					die();
					//TODO make this error nicer
				}
			}
		}

		while ($period = $this->db->fetch($dataQuery)) {
			// Calculate Gain Difference
			$period['gain_delta'] = 0;
			$period['value_delta'] = 0;
			if (isset($last)) {
				$period['gain_delta'] = number_format($period['gain'] - $last['gain'], 2);
				$period['value_delta'] = number_format($period['asset_total'] - $last['asset_total'] - $period['gain_delta'], 2);
			}
			if ($period['deposit_total'] > 0) {
				$period['growth'] = (($period['gain'] / $period['deposit_total'])*100);
			} else {
				$period['growth'] = 0;
			}
			$period['growth_str'] = number_format($period['growth'],2);

			$vars['periodData'][] = $period;
			$last = $period;

		}

		$last['deposit_str'] = $this->prettify($last['deposit_total']);
		$last['asset_str'] = $this->prettify($last['asset_total']);
		$last['gain_str'] = $this->prettify($last['gain']);
		$vars['mostRecent'] = $last;

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
