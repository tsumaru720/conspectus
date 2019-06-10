<?php

class Document extends Theme {

	protected $pageTitle = 'Breakdown';
	private $db = null;

	public function __construct(&$main, &$twig, $vars) {

		$vars['page_title'] = $this->pageTitle;

		$this->db = $main->getDB();
		$page = $main->getPage();

		$logQuery = $this->db->query("SELECT asset_id, 
								description,
								deposit_value,
								asset_value,
								(asset_value - deposit_value) AS gain,
								DATE_FORMAT(epoch, '%b %Y') AS period,
								EXTRACT(YEAR_MONTH FROM epoch) AS yearMonth
								FROM asset_log
								LEFT JOIN asset_list
								ON asset_log.asset_id = asset_list.id
								ORDER BY yearMonth ASC, description ASC");

		while ($log = $this->db->fetch($logQuery)) {
			$vars['labels'][$log['yearMonth']] = $log['period'];
			
			//$vars['log'][$log['yearMonth']][] = array('name' => $log['description'], 'value' => $log['asset_value']);
			$vars['log'][$log['asset_id']]['name'] = $log['description'];
			$vars['log'][$log['asset_id']]['data'][$log['yearMonth']] = $log['asset_value'];
			
			$last['log'][$log['asset_id']]['name'] = $log['description'];
			$last['log'][$log['asset_id']]['value'] = $log['asset_value'];
		}

		$vars['mostRecent'] = $last['log'];

		foreach ($vars['log'] as $key => $entry) {
			foreach ($vars['labels'] as $period => $v) {
				if (!array_key_exists($period, $entry['data'])) {
					$vars['log'][$key]['data'][$period] = 0;
					ksort($vars['log'][$key]['data']);
				}
			}
		}
		
//$this->dump($vars);

		$this->vars = $vars;
		$this->document = $twig->load('breakdown.html');
	}

}
/*
  ["labels"]=>
  array(6) {
    [201901]=>
    string(8) "Jan 2019"
    [201902]=>
    string(8) "Feb 2019"
    [201903]=>
    string(8) "Mar 2019"
    [201904]=>
    string(8) "Apr 2019"
    [201905]=>
    string(8) "May 2019"
    [201906]=>
    string(8) "Jun 2019"
  }


  array(14) {
    [39]=>
    array(2) {
      ["name"]=>
      string(14) "AEGON (iomart)"
      ["data"]=>
      array(6) {
        [201901]=>
        string(7) "2918.67"
        [201902]=>
        string(7) "3159.21"
        [201903]=>
        string(7) "3324.69"
        [201904]=>
        string(7) "3515.32"
        [201905]=>
        string(7) "3742.87"
        [201906]=>
        string(7) "3851.30"
      }
    }
    [3]=>
    array(2) {
      ["name"]=>
      string(18) "HSBC Regular Saver"
      ["data"]=>
      array(6) {
        [201901]=>
        string(6) "750.00"
        [201902]=>
        string(7) "1000.00"
        [201903]=>
        string(7) "1250.00"
        [201904]=>
        string(7) "1500.00"
        [201905]=>
        string(7) "1750.00"
        [201906]=>
        string(7) "2000.00"
      }
    }
*/
