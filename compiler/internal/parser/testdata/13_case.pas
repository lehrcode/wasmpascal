program CaseAnweisung;
var
  n: Integer;
  c: Char;
begin
  ReadLn(n);
  case n of
    0: WriteLn('null');
    1, 3, 5, 7, 9: WriteLn('ungerade Ziffer');
    2, 4, 6, 8: WriteLn('gerade Ziffer');
    10..99: WriteLn('zweistellig');
    -9..-1: ;
  else
    WriteLn('sonstwas');
    WriteLn('wirklich')
  end;
  ReadLn(c);
  case UpCase(c) of
    'A'..'Z': WriteLn('Buchstabe');
    '0'..'9': WriteLn('Ziffer');
    ' ', #9: WriteLn('Leerraum');
    ^M, ^J: WriteLn('Zeilenende')
  end;
  case n mod 3 of
    0: begin
         WriteLn('durch 3 teilbar');
         n := n div 3
       end;
    1, 2:
  end
end.
