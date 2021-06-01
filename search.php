<?php

$dizin = "C:/xampp"; //verilerin bulunduğu klasör


$search_key ="";

$css =0;
echo '<div class="title">'; if($search_key != "") { echo '"<b><i>'.$search_key.'</i></b>" ile ilgili sonuçlar:'; } else { echo 'Tüm dosyalar:'; } echo '</div>';

//ara_dizini ($dizin);
// Buradan alınmıştır https://stackoverflow.com/questions/5501427/php-filesize-mb-kb-conversion/56361757#56361757
function filesize_formatted($path)
{
    $size = filesize($path);
    $units = array( 'B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB');
    $power = $size > 0 ? floor(log($size, 1024)) : 0;
    return number_format($size / pow(1024, $power), 2, '.', ',') . ' ' . $units[$power];
}
//https://stackoverflow.com/questions/2263311/using-a-variable-as-an-operator
function checkOperator($value1, $operator, $value2) {
    switch ($operator) {
        case '<': // Less than
            return $value1 < $value2;
        case '<=': // Less than or equal to
            return $value1 <= $value2;
        case '>': // Greater than
            return $value1 > $value2;
        case '>=': // Greater than or equal to
            return $value1 >= $value2;
        case '==': // Equal
            return $value1 == $value2;
        case '===': // Identical
            return $value1 === $value2;
        case '!==': // Not Identical
            return $value1 !== $value2;
        case '!=': // Not equal
        case '<>': // Not equal
            return $value1 != $value2;
        case '||': // Or
        case 'or': // Or
           return $value1 || $value2;
        case '&&': // And
        case 'and': // And
           return $value1 && $value2;
        case 'xor': // Or
           return $value1 xor $value2;  
        default:
            return FALSE;
 } // end switch
}



function ara_dizini ($dizin){
    global $search_key;
    global $css;
    if ($handle = @opendir("$dizin")) {

        while (false !== ($file = readdir($handle))) {
       
            //Arama terimi ve dosya adı karşılaştırması
            $search_word = preg_match("/$search_key/i", "$file");
            //is_file($dizin."/".$file) && 
    
            if(is_file($dizin."/".$file) &&  $search_word) { //eğer bir dosya ise ve arama terimini içeriyorsa
    
                 $filename = str_replace("/$search_key/i","<i><b>$search_key</b></i>",$file);
    
                $class = ($css % 2) ? "satir1" : "satir2"; // her satira farklı class
    
                echo '
                <div class="'.$class.'"><a href="'.$dizin.'/'.$file.'">'.$filename.'  size: '.filesize_formatted($dizin."/".$file).'</a> </div>';
    
                $css ++;
           }
           else{
               if($file!="." && $file!=".."){
                    if(is_dir($dizin."/".$file)){
                         ara_dizini($dizin."/".$file);
                    } 
                   
               }
           
            
           }
    
        } //while end
    
        closedir($handle);
    }

}

// function checksize($value1,$value2) {
//     switch ($value2) {
//         case 'B':             
//             return $value1 == $value2;
//         case 'KB':
            
//             return ($value1 == $value2)||($value1=='B');
//         case 'MB':
        
//             return ($value1 == $value2)||($value1=='B')||($value1=='KB');
//         case 'GB':
    
//             return ($value1 == $value2)||($value1=='B')||($value1=='KB')||($value1=='MB');
//         case 'TB':
//             return ($value1 == $value2)||($value1=='B')||($value1=='KB')||($value1=='MB')||($value1=='GB');
//         default:
//             return FALSE;
//  } // end switch
// }

// function checksizetype($value1,$value2,$item1,$op,$item2) {
//     switch ($value2) {
//         case 'B':             
//             return checkOperator($item1,$op,$item2);
//         case 'KB':
            
//             return (checkOperator($item1,$op,$item2)&&($value1 == $value2))||($value1=='B');
//         case 'MB':
        
//             return (checkOperator($item1,$op,$item2)&&($value1 == $value2))||($value1=='B')||($value1=='KB');
//         case 'GB':
    
//             return (checkOperator($item1,$op,$item2)&&($value1 == $value2))||($value1=='B')||($value1=='KB')||($value1=='MB');
//         case 'TB':
//             return (checkOperator($item1,$op,$item2)&&($value1 == $value2))||($value1=='B')||($value1=='KB')||($value1=='MB')||($value1=='GB');
//         default:
//             return FALSE;
//  } // end switch
// }


function checksizeall($value1,$value2,$item1,$op,$item2) {
    switch ($value2) {
        case 'B':             
            if(($value1 == $value2)&&checkOperator($item1,$op,$item2))
            return true;
            elseif($op=='!=')
            return !(($value1 == $value2)&&checkOperator($item1,'==',$item2));
            elseif($op=='>'||$op=='>=')
            return $value1=='KB' ||$value1=='MB' || $value1=='GB' || $value1=='TB' ;           
            else{
                return false;
            }        
        case 'KB':
            if(($value1 == $value2)&&checkOperator($item1,$op,$item2))
            return true;
            elseif($op=='!=')
            return !(($value1 == $value2)&&checkOperator($item1,'==',$item2));
            elseif($op=='<'||$op=='<=')
            return $value1=='B';
            elseif($op=='>'||$op=='>=')
            return $value1=='MB' || $value1=='GB' || $value1=='TB';
            else{
                return false;
            }
           
        case 'MB':
            if(($value1 == $value2)&&checkOperator($item1,$op,$item2))
            return true;
            elseif($op=='!=')
            return !(($value1 == $value2)&&checkOperator($item1,'==',$item2));
            elseif($op=='<'||$op=='<=')
            return $value1=='B' ||$value1=='KB';
            elseif($op=='>'||$op=='>=')
            return $value1=='GB' || $value1=='TB';
            else{
                return false;
            }
        case 'GB':
            if(($value1 == $value2)&&checkOperator($item1,$op,$item2))
            return true;
            elseif($op=='!=')
            return !(($value1 == $value2)&&checkOperator($item1,'==',$item2));
            elseif($op=='<'||$op=='<=')
            return $value1=='B' ||$value1=='KB' ||$value1=='MB' ;
            elseif($op=='>'||$op=='>=')
            return $value1=='TB';
            else{
                return false;
            }
        case 'TB':
            if(($value1 == $value2)&&checkOperator($item1,$op,$item2))
            return true;
            elseif($op=='!=')
            return !(($value1 == $value2)&&checkOperator($item1,'==',$item2));
            elseif($op=='<'||$op=='<=')
            return $value1=='B' ||$value1=='KB' ||$value1=='MB' || $value1=='GB'  ;           
            else{
                return false;
            }
        default:
            return FALSE;
 } // end switch
}

$search_size;
function ara_dizini_boyut ($dizin){
    global $search_key;
    global $search_size;
    global $size_type;
    global $size_equals;

    global $css;
    if ($handle = @opendir("$dizin")) {

        while (false !== ($file = readdir($handle))) {
       
            //Arama terimi ve dosya adı karşılaştırması
           // $search_word = preg_match("/$search_key/i", "$file");
            //is_file($dizin."/".$file) && 
            $size=explode(" ",filesize_formatted($dizin."/".$file));
    
            if(is_file($dizin."/".$file) && checksizeall($size[1],$size_type,$size[0],$size_equals,$search_size)) { //eğer bir dosya ise ve arama terimini içeriyorsa
    
                 $filename = str_replace("/$search_key/i","<i><b>$search_key</b></i>",$file);
    
                $class = ($css % 2) ? "satir1" : "satir2"; // her satira farklı class
    
                echo '
                <div class="'.$class.'"><a href="'.$dizin.'/'.$file.'">'.$filename.'  size: '.filesize_formatted($dizin."/".$file).'</a> </div>';
    
                $css ++;
           }
           else{
               if($file!="." && $file!=".."){
                    if(is_dir($dizin."/".$file)){
                         ara_dizini_boyut($dizin."/".$file);
                    } 
                   
               }
           
            
           }
    
        } //while end
    
        closedir($handle);
    }
    

}

function ara_dizini_type ($dizin){
    global $search_type;
    global $search_key;

    global $css;
    if ($handle = @opendir("$dizin")) {

        while (false !== ($file = readdir($handle))) {
       
            //Arama terimi ve dosya adı karşılaştırması
            //$search_word = preg_match("/$search_key/i", "$file");
            //is_file($dizin."/".$file) && 
            $ext = pathinfo($dizin."/".$file, PATHINFO_EXTENSION);


            if(is_file($dizin."/".$file) &&  $ext==$search_type) { //eğer bir dosya ise ve arama terimini içeriyorsa
    
                 $filename = str_replace("/$search_key/i","<i><b>$search_key</b></i>",$file);
    
                $class = ($css % 2) ? "satir1" : "satir2"; // her satira farklı class
                echo '
                <div class="'.$class.'"><a href="'.$dizin.'/'.$file.'">'.$filename.'  size: '.filesize_formatted($dizin."/".$file).'.....uzantı:'.$ext.'</a> </div>';
    
                $css ++;
           }
           else{
               if($file!="." && $file!=".."){
                    if(is_dir($dizin."/".$file)){
                         ara_dizini_type($dizin."/".$file);
                    } 
                   
               }
           
            
           }
    
        } //while end
    
        closedir($handle);
    }

}

           
// Buradan yararlanılmıştır https://www.egonomik.com/php-klasor-icerisinde-arama-yapmak/
function ara_dizini_date ($dizin){
        global $search_key;
        global $css;
        if ($handle = @opendir("$dizin")) {
    
            while (false !== ($file = readdir($handle))) {
           
                //Arama terimi ve dosya adı karşılaştırması
                $search_word = preg_match("/$search_key/i", "$file");
                //is_file($dizin."/".$file) && 
                $tarih=date ("F d Y H:i:s.", filemtime($dizin."/".$file));
        
                if(is_file($dizin."/".$file) &&  $search_word) { //eğer bir dosya ise ve arama terimini içeriyorsa
        
                     $filename = str_replace("/$search_key/i","<i><b>$search_key</b></i>",$file);
        
                    $class = ($css % 2) ? "satir1" : "satir2"; // her satira farklı class
        
                    echo '
                    <div class="'.$class.'"><a href="'.$dizin.'/'.$file.'">'.$filename.'  size: '.filesize_formatted($dizin."/".$file).'Degistirme tarihi: '.$tarih.'</a> </div>';
        
                    $css ++;
               }
               else{
                   if($file!="." && $file!=".."){
                        if(is_dir($dizin."/".$file)){
                             ara_dizini($dizin."/".$file);
                        } 
                       
                   }
               
                
               }
        
            } //while end
        
            closedir($handle);
        }
    
    
}
// Buradan yararlanılmıştır https://www.php.net/manual/en/function.stat.php
function ara_dizini_stat ($dizin){
    global $search_key;
    global $css;
    if ($handle = @opendir("$dizin")) {

        while (false !== ($file = readdir($handle))) {
             
            //Arama terimi ve dosya adı karşılaştırması
            $search_word = preg_match("/$search_key/i", "$file");
            //is_file($dizin."/".$file) && 
            $stat = substr(sprintf("%o",fileperms($dizin."/".$file)), -4);
            if(is_file($dizin."/".$file) &&  $search_word) { //eğer bir dosya ise ve arama terimini içeriyorsa
    
                 $filename = str_replace("/$search_key/i","<i><b>$search_key</b></i>",$file);
    
                $class = ($css % 2) ? "satir1" : "satir2"; // her satira farklı class
    
                echo '
                <div class="'.$class.'"><a href="'.$dizin.'/'.$file.'">'.$filename.'  size: '.filesize_formatted($dizin."/".$file).'User: '.' Group: '.$stat.'</a> </div>';
    
                $css ++;
           }
           else{
               if($file!="." && $file!=".."){
                    if(is_dir($dizin."/".$file)){
                         ara_dizini_stat($dizin."/".$file);
                    } 
                   
               }
           
            
           }
    
        } //while end
    
        closedir($handle);
    }

}





//dosya uzantısı veriyor

//$ext = pathinfo($filename, PATHINFO_EXTENSION);

// if ($handle = opendir("$dizin") or die ("Dizin acilamadi!")) {

//     while (false !== ($file = readdir($handle))) {
   
//         //Arama terimi ve dosya adı karşılaştırması
//         $search_word = preg_match("/$search_key/i", "$file");
//         //is_file($dizin."/".$file) && 

//         if($search_word) { //eğer bir dosya ise ve arama terimini içeriyorsa

//              $filename = str_replace("/$search_key/i","<i><b>$search_key</b></i>",$file);

//             $class = ($css % 2) ? "satir1" : "satir2"; // her satira farklı class

//             echo '
//             <div class="'.$class.'"><a href="'.$dizin.'/'.$file.'">'.$filename.'</a> </div>';

//             $css ++;
//        }

//     } //while end

//     closedir($handle);
// }
