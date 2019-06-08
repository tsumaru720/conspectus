<?php

class Document extends Theme {

	protected $pageTitle = 'Dashboard';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();
		$data = array(':asset_id' => $vars['asset_id']);
		
		if ($vars['asset_id'] > 0) {
			$q = $this->db->query("SELECT description FROM asset_list WHERE id = :asset_id;", $data);
			if ($asset = $this->db->fetch($q)) {
				$this->pageTitle = "Asset View - ".$asset['description'];
				$vars['page_title'] = $this->pageTitle;
			} else {
				echo "invalid asset";
				die();
				//TODO make this error nicer
			}
		}

		$q = $this->db->query("SELECT SUM(deposit_value) AS deposit_total,
								SUM(asset_value) AS asset_total,
								SUM(asset_value - deposit_value) AS gain,
								DATE_FORMAT(epoch, '%b %Y') AS period,
								EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
								FROM asset_log
								WHERE asset_id ".$vars['modifier']." :asset_id
								GROUP BY period, yearMonth
								ORDER BY yearMonth ASC", $data);

		while ($period = $this->db->fetch($q)) {
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
		$this->document = $twig->load('asset_view.html');
	}

	private function prettify($value) {
		return ($value < 0 ? '-£'.number_format($value * -1, 2) : '£'.number_format($value, 2));
		//TODO some kind of configurable currency
	}

}
//TODO Deal with no results (Fresh install?)
//TODO fill in missing months as zero
